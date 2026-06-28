// Package crypt implements file and filename encryption that is compatible
// with rclone's "crypt" backend defaults.
//
// The format matches rclone:
//   - scrypt (N=16384, r=8, p=1) for key material derivation.
//   - NaCl SecretBox (XSalsa20 + Poly1305) for file contents, in 64 KiB blocks.
//   - AES-256 in EME mode for filenames, with a configurable output encoding.
//
// When no salt is supplied rclone uses a fixed default salt, which this
// package mirrors so output is interchangeable with rclone itself.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rfjakob/eme"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/scrypt"
)

// File format constants, identical to rclone's crypt backend.
const (
	fileMagic       = "RCLONE\x00\x00"
	fileMagicSize   = len(fileMagic)
	fileNonceSize   = 24
	fileHeaderSize  = fileMagicSize + fileNonceSize
	blockHeaderSize = secretbox.Overhead // 16 bytes Poly1305 tag
	blockDataSize   = 64 * 1024
	blockSize       = blockHeaderSize + blockDataSize
)

// Key material sizes.
const (
	keySize       = 32
	nameKeySize   = 32
	nameTweakSize = 16
)

// scrypt parameters used by rclone.
const (
	scryptN = 16384
	scryptR = 8
	scryptP = 1
)

// defaultSalt is the fixed salt rclone uses when no salt (password2) is given.
var defaultSalt = []byte{
	0xA8, 0x0D, 0xF4, 0x3A, 0x8F, 0xBD, 0x03, 0x08,
	0xA7, 0xCA, 0xB8, 0x3E, 0x58, 0x1F, 0x86, 0xB1,
}

// NameEncoding selects how encrypted filename bytes are rendered as text.
type NameEncoding int

const (
	// EncodingBase32 is rclone's default: lowercase base32 (RFC 4648 hex
	// alphabet) with padding stripped. Case-insensitive, safe on Windows.
	EncodingBase32 NameEncoding = iota
	// EncodingBase64 uses URL-safe base64 without padding. Shorter names,
	// but case-sensitive.
	EncodingBase64
)

// String returns the canonical flag value for the encoding.
func (e NameEncoding) String() string {
	switch e {
	case EncodingBase64:
		return "base64"
	default:
		return "base32"
	}
}

// ParseEncoding converts a user supplied string into a NameEncoding.
func ParseEncoding(s string) (NameEncoding, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "base32":
		return EncodingBase32, nil
	case "base64":
		return EncodingBase64, nil
	default:
		return 0, fmt.Errorf("unsupported filename encoding %q (supported: base32, base64)", s)
	}
}

// Cipher performs rclone-compatible encryption and decryption.
type Cipher struct {
	dataKey   [keySize]byte
	nameKey   [nameKeySize]byte
	nameTweak [nameTweakSize]byte
	block     cipher.Block
	encoding  NameEncoding
}

// NewCipher derives the key material from password and salt and returns a
// ready to use Cipher. An empty salt selects rclone's default salt.
func NewCipher(password, salt string, encoding NameEncoding) (*Cipher, error) {
	if password == "" {
		return nil, errors.New("password must not be empty")
	}
	c := &Cipher{encoding: encoding}

	saltBytes := defaultSalt
	if salt != "" {
		saltBytes = []byte(salt)
	}

	key, err := scrypt.Key([]byte(password), saltBytes, scryptN, scryptR, scryptP, keySize+nameKeySize+nameTweakSize)
	if err != nil {
		return nil, fmt.Errorf("deriving key: %w", err)
	}
	copy(c.dataKey[:], key[:keySize])
	copy(c.nameKey[:], key[keySize:keySize+nameKeySize])
	copy(c.nameTweak[:], key[keySize+nameKeySize:])

	c.block, err = aes.NewCipher(c.nameKey[:])
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// Filename encryption
// ---------------------------------------------------------------------------

func (c *Cipher) encodeName(in []byte) string {
	if c.encoding == EncodingBase64 {
		return base64.RawURLEncoding.EncodeToString(in)
	}
	encoded := base32.HexEncoding.EncodeToString(in)
	return strings.ToLower(strings.TrimRight(encoded, "="))
}

func (c *Cipher) decodeName(in string) ([]byte, error) {
	if c.encoding == EncodingBase64 {
		return base64.RawURLEncoding.DecodeString(in)
	}
	encoded := strings.ToUpper(in)
	if pad := len(encoded) % 8; pad != 0 {
		encoded += strings.Repeat("=", 8-pad)
	}
	return base32.HexEncoding.DecodeString(encoded)
}

// EncryptFileName encrypts a filename. Path separators ("/") are preserved and
// each segment is encrypted independently, matching rclone.
func (c *Cipher) EncryptFileName(name string) string {
	segments := strings.Split(name, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		padded := pkcs7Pad(aes.BlockSize, []byte(seg))
		ciphertext := eme.Transform(c.block, c.nameTweak[:], padded, eme.DirectionEncrypt)
		segments[i] = c.encodeName(ciphertext)
	}
	return strings.Join(segments, "/")
}

// DecryptFileName reverses EncryptFileName.
func (c *Cipher) DecryptFileName(name string) (string, error) {
	segments := strings.Split(name, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		rawCipher, err := c.decodeName(seg)
		if err != nil {
			return "", fmt.Errorf("decoding filename: %w", err)
		}
		if len(rawCipher) == 0 || len(rawCipher)%aes.BlockSize != 0 {
			return "", errors.New("encrypted filename is not a multiple of the block size")
		}
		padded := eme.Transform(c.block, c.nameTweak[:], rawCipher, eme.DirectionDecrypt)
		plaintext, err := pkcs7Unpad(aes.BlockSize, padded)
		if err != nil {
			return "", fmt.Errorf("decrypting filename (wrong password/salt?): %w", err)
		}
		segments[i] = string(plaintext)
	}
	return strings.Join(segments, "/"), nil
}

// ---------------------------------------------------------------------------
// File content encryption
// ---------------------------------------------------------------------------

// nonce is the 24 byte XSalsa20 nonce, incremented per block.
type nonce [fileNonceSize]byte

// increment adds one to the nonce treated as a little-endian number.
func (n *nonce) increment() {
	for i := range n {
		n[i]++
		if n[i] != 0 {
			return
		}
	}
}

// EncryptData reads plaintext from in and writes the rclone crypt stream to out.
func (c *Cipher) EncryptData(in io.Reader, out io.Writer) error {
	var n nonce
	if _, err := io.ReadFull(rand.Reader, n[:]); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}

	if _, err := out.Write([]byte(fileMagic)); err != nil {
		return err
	}
	if _, err := out.Write(n[:]); err != nil {
		return err
	}

	buf := make([]byte, blockDataSize)
	encBuf := make([]byte, 0, blockSize)
	for {
		nr, err := io.ReadFull(in, buf)
		if nr > 0 {
			encBuf = secretbox.Seal(encBuf[:0], buf[:nr], (*[fileNonceSize]byte)(&n), &c.dataKey)
			if _, werr := out.Write(encBuf); werr != nil {
				return werr
			}
			n.increment()
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
	}
}

// DecryptData reads an rclone crypt stream from in and writes plaintext to out.
func (c *Cipher) DecryptData(in io.Reader, out io.Writer) error {
	header := make([]byte, fileHeaderSize)
	if _, err := io.ReadFull(in, header); err != nil {
		return fmt.Errorf("reading file header (truncated or not encrypted?): %w", err)
	}
	if string(header[:fileMagicSize]) != fileMagic {
		return errors.New("input is not an rclone-encrypted file (bad magic header)")
	}

	var n nonce
	copy(n[:], header[fileMagicSize:])

	buf := make([]byte, blockSize)
	plainBuf := make([]byte, 0, blockDataSize)
	for {
		nr, err := io.ReadFull(in, buf)
		if nr > 0 {
			if nr <= blockHeaderSize {
				return errors.New("corrupted input: encrypted block too short")
			}
			var ok bool
			plainBuf, ok = secretbox.Open(plainBuf[:0], buf[:nr], (*[fileNonceSize]byte)(&n), &c.dataKey)
			if !ok {
				return errors.New("decryption failed: wrong password/salt or corrupted data")
			}
			if _, werr := out.Write(plainBuf); werr != nil {
				return werr
			}
			n.increment()
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
	}
}

// ---------------------------------------------------------------------------
// PKCS#7 padding (matches rclone's lib/pkcs7)
// ---------------------------------------------------------------------------

func pkcs7Pad(blockSize int, buf []byte) []byte {
	padding := blockSize - (len(buf) % blockSize)
	for range padding {
		buf = append(buf, byte(padding))
	}
	return buf
}

func pkcs7Unpad(blockSize int, buf []byte) ([]byte, error) {
	length := len(buf)
	if length == 0 {
		return nil, errors.New("pkcs7: no data")
	}
	if length%blockSize != 0 {
		return nil, errors.New("pkcs7: data not a multiple of block size")
	}
	padding := int(buf[length-1])
	if padding == 0 || padding > blockSize || padding > length {
		return nil, errors.New("pkcs7: invalid padding")
	}
	for i := range padding {
		if buf[length-1-i] != byte(padding) {
			return nil, errors.New("pkcs7: invalid padding")
		}
	}
	return buf[:length-padding], nil
}
