package crypt

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPassword = "Testpassword1"

// Known rclone-generated vectors (default salt, password "Testpassword1").
// These prove byte-for-byte compatibility with rclone itself.
var nameVectors = []struct {
	encoding  NameEncoding
	encrypted string
	plain     string
}{
	{EncodingBase32, "kr9tu4e1da4u3nifdd99g9tf5o", "TEST_FILE.txt"},
	{EncodingBase64, "Iyxcijgc9bp3o5Y0npW6xqUvwWNcc3MA4SadB0sR6cY", "TEST_FILE BASE64.txt"},
}

func TestDecryptKnownFileNames(t *testing.T) {
	for _, v := range nameVectors {
		c, err := NewCipher(testPassword, "", v.encoding)
		if err != nil {
			t.Fatalf("NewCipher: %v", err)
		}
		got, err := c.DecryptFileName(v.encrypted)
		if err != nil {
			t.Fatalf("DecryptFileName(%q): %v", v.encrypted, err)
		}
		if got != v.plain {
			t.Errorf("DecryptFileName(%q) = %q, want %q", v.encrypted, got, v.plain)
		}
	}
}

func TestEncryptKnownFileNames(t *testing.T) {
	// Filename encryption is deterministic (EME), so it must reproduce the
	// exact rclone-generated encrypted names.
	for _, v := range nameVectors {
		c, err := NewCipher(testPassword, "", v.encoding)
		if err != nil {
			t.Fatalf("NewCipher: %v", err)
		}
		got := c.EncryptFileName(v.plain)
		if got != v.encrypted {
			t.Errorf("EncryptFileName(%q) = %q, want %q", v.plain, got, v.encrypted)
		}
	}
}

func TestFileNameRoundTrip(t *testing.T) {
	for _, enc := range []NameEncoding{EncodingBase32, EncodingBase64} {
		c, err := NewCipher(testPassword, "somesalt", enc)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"a", "hello world.txt", "nested/dir/file.bin", "διεθνής.dat"} {
			encName := c.EncryptFileName(name)
			dec, err := c.DecryptFileName(encName)
			if err != nil {
				t.Fatalf("DecryptFileName(%q): %v", encName, err)
			}
			if dec != name {
				t.Errorf("round trip (%s): got %q want %q", enc, dec, name)
			}
		}
	}
}

func TestDataRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		salt string
		size int
	}{
		{"empty no salt", "", 0},
		{"small no salt", "", 11},
		{"small with salt", "my-custom-salt", 11},
		{"one block boundary", "", blockDataSize},
		{"multi block", "", blockDataSize*2 + 123},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewCipher(testPassword, tc.salt, EncodingBase32)
			if err != nil {
				t.Fatal(err)
			}
			plain := make([]byte, tc.size)
			for i := range plain {
				plain[i] = byte(i % 251)
			}

			var enc bytes.Buffer
			if err := c.EncryptData(bytes.NewReader(plain), &enc); err != nil {
				t.Fatalf("EncryptData: %v", err)
			}

			var dec bytes.Buffer
			if err := c.DecryptData(bytes.NewReader(enc.Bytes()), &dec); err != nil {
				t.Fatalf("DecryptData: %v", err)
			}
			if !bytes.Equal(dec.Bytes(), plain) {
				t.Errorf("round trip mismatch: got %d bytes, want %d", dec.Len(), len(plain))
			}
		})
	}
}

func TestWrongPasswordFailsToDecrypt(t *testing.T) {
	good, _ := NewCipher(testPassword, "", EncodingBase32)
	var enc bytes.Buffer
	if err := good.EncryptData(strings.NewReader("secret data"), &enc); err != nil {
		t.Fatal(err)
	}
	bad, _ := NewCipher("wrong-password", "", EncodingBase32)
	var out bytes.Buffer
	if err := bad.DecryptData(bytes.NewReader(enc.Bytes()), &out); err == nil {
		t.Error("expected decryption to fail with wrong password, got nil error")
	}
}

func TestWrongSaltFailsToDecrypt(t *testing.T) {
	good, _ := NewCipher(testPassword, "salt-a", EncodingBase32)
	var enc bytes.Buffer
	if err := good.EncryptData(strings.NewReader("secret data"), &enc); err != nil {
		t.Fatal(err)
	}
	bad, _ := NewCipher(testPassword, "salt-b", EncodingBase32)
	var out bytes.Buffer
	if err := bad.DecryptData(bytes.NewReader(enc.Bytes()), &out); err == nil {
		t.Error("expected decryption to fail with wrong salt, got nil error")
	}
}

func TestDecryptRejectsBadMagic(t *testing.T) {
	c, _ := NewCipher(testPassword, "", EncodingBase32)
	var out bytes.Buffer
	err := c.DecryptData(strings.NewReader("not an rclone file at all....................."), &out)
	if err == nil {
		t.Error("expected error for bad magic header")
	}
}

func TestEmptyPasswordRejected(t *testing.T) {
	if _, err := NewCipher("", "", EncodingBase32); err == nil {
		t.Error("expected error for empty password")
	}
}

func TestParseEncoding(t *testing.T) {
	cases := map[string]NameEncoding{"": EncodingBase32, "base32": EncodingBase32, "BASE32": EncodingBase32, "base64": EncodingBase64}
	for in, want := range cases {
		got, err := ParseEncoding(in)
		if err != nil || got != want {
			t.Errorf("ParseEncoding(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseEncoding("base99"); err == nil {
		t.Error("expected error for unknown encoding")
	}
}

// TestDecryptKnownFileContents decrypts the committed sample files (if present)
// and verifies the plaintext, proving end-to-end rclone compatibility.
func TestDecryptKnownFileContents(t *testing.T) {
	const wantSubstring = "umbrella top kit charge tobacco"
	for _, f := range []string{"kr9tu4e1da4u3nifdd99g9tf5o", "Iyxcijgc9bp3o5Y0npW6xqUvwWNcc3MA4SadB0sR6cY"} {
		path := filepath.Join("..", "..", f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("sample file %s not available: %v", f, err)
		}
		c, _ := NewCipher(testPassword, "", EncodingBase32)
		var out bytes.Buffer
		if err := c.DecryptData(bytes.NewReader(data), &out); err != nil {
			t.Fatalf("DecryptData(%s): %v", f, err)
		}
		if !strings.Contains(out.String(), wantSubstring) {
			t.Errorf("decrypted %s does not contain expected bip39 words; got %q", f, out.String())
		}
	}
}
