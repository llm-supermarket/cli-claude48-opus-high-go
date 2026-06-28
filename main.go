// Command cli-claude48-opus-high-go encrypts and decrypts files using rclone's
// crypt defaults (scrypt key derivation, NaCl SecretBox file contents, AES-256
// EME filenames). See the README for usage examples.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/llm-supermarket/cli-claude48-opus-high-go/internal/crypt"
	"golang.org/x/term"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

const appName = "cli-claude48-opus-high-go"

// Environment variables used to supply secrets without exposing them on the
// command line or in shell history.
const (
	envPassword = "RCLONE_ENCRYPT_PASSWORD"
	envSalt     = "RCLONE_ENCRYPT_SALT"
)

// stdin is a single shared reader so sequential prompts (password then salt)
// don't lose buffered bytes when input is piped.
var stdin = bufio.NewReader(os.Stdin)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no command given")
	}

	switch args[0] {
	case "encrypt":
		return runCommand("encrypt", args[1:])
	case "decrypt":
		return runCommand("decrypt", args[1:])
	case "version", "--version", "-v":
		fmt.Printf("%s %s\n", appName, version)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `%s %s - rclone-compatible file encryption

Usage:
  %s encrypt -i <input> [-o <output>] [flags]
  %s decrypt -i <input> [-o <output>] [flags]
  %s version

Flags:
  -i, --input-file       Input file to encrypt or decrypt (required)
  -o, --output-file      Output file (optional; the filename is encrypted or
                         decrypted automatically when omitted)
      --filename-encoding  Encoding for the encrypted filename: base32 (default,
                         rclone's default) or base64
      --password         Password (INSECURE - see warning below; prefer the
                         %s environment variable or the interactive prompt)
      --salt             Optional salt (rclone "password2"). When omitted,
                         rclone's default salt is used. May also be supplied via
                         the %s environment variable.

Security note:
  Passing --password puts your password in your shell history and in the
  process list where other users may see it. Prefer the interactive prompt or
  set the %s environment variable. If you do use --password,
  clear it from your shell history afterwards (e.g. 'history -d <n>' for bash,
  or 'Clear-History' / remove the line from PSReadLine history for PowerShell).
`, appName, version, appName, appName, appName, envPassword, envSalt, envPassword)
}

func runCommand(mode string, args []string) error {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)

	var inputFile, outputFile, encodingStr, password, salt string
	fs.StringVar(&inputFile, "i", "", "input file")
	fs.StringVar(&inputFile, "input-file", "", "input file")
	fs.StringVar(&outputFile, "o", "", "output file")
	fs.StringVar(&outputFile, "output-file", "", "output file")
	fs.StringVar(&encodingStr, "filename-encoding", "base32", "filename encoding: base32 or base64")
	fs.StringVar(&password, "password", "", "password (insecure)")
	fs.StringVar(&salt, "salt", "", "optional salt")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if inputFile == "" {
		return errors.New("input file is required (-i / --input-file)")
	}

	encoding, err := crypt.ParseEncoding(encodingStr)
	if err != nil {
		return err
	}

	// "salt" was explicitly set on the command line?
	saltFlagSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "salt" {
			saltFlagSet = true
		}
	})

	resolvedPassword, err := resolvePassword(password)
	if err != nil {
		return err
	}
	resolvedSalt, err := resolveSalt(saltFlagSet, salt)
	if err != nil {
		return err
	}

	cipher, err := crypt.NewCipher(resolvedPassword, resolvedSalt, encoding)
	if err != nil {
		return err
	}

	// Determine output path, transforming the filename when -o is omitted.
	if outputFile == "" {
		outputFile, err = defaultOutputPath(mode, inputFile, cipher)
		if err != nil {
			return err
		}
	}

	if err := process(mode, inputFile, outputFile, cipher); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "%sed %s -> %s\n", mode, inputFile, outputFile)
	return nil
}

func defaultOutputPath(mode, inputFile string, cipher *crypt.Cipher) (string, error) {
	dir := filepath.Dir(inputFile)
	base := filepath.Base(inputFile)
	var newBase string
	if mode == "encrypt" {
		newBase = cipher.EncryptFileName(base)
	} else {
		var err error
		newBase, err = cipher.DecryptFileName(base)
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, newBase), nil
}

func process(mode, inputFile, outputFile string, cipher *crypt.Cipher) (err error) {
	if sameFile(inputFile, outputFile) {
		return errors.New("input and output file are the same; refusing to overwrite the input")
	}

	in, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
		if err != nil {
			// Don't leave a half-written / corrupt output file behind.
			os.Remove(outputFile)
		}
	}()

	if mode == "encrypt" {
		return cipher.EncryptData(in, out)
	}
	return cipher.DecryptData(in, out)
}

// sameFile reports whether two paths refer to the same location. It falls back
// to a cleaned-path comparison when the absolute path cannot be resolved.
func sameFile(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return absA == absB
}

// resolvePassword resolves the password from the flag, environment, or prompt.
func resolvePassword(flagVal string) (string, error) {
	if flagVal != "" {
		fmt.Fprintf(os.Stderr,
			"WARNING: --password exposes your password in shell history and the process list.\n"+
				"         Prefer the %s environment variable or the interactive prompt,\n"+
				"         and clear this command from your shell history afterwards.\n",
			envPassword)
		return flagVal, nil
	}
	if env, ok := os.LookupEnv(envPassword); ok && env != "" {
		return env, nil
	}
	pw, err := readSecret("Password: ")
	if err != nil {
		return "", err
	}
	if pw == "" {
		return "", errors.New("password must not be empty")
	}
	return pw, nil
}

// resolveSalt resolves the optional salt from the flag, environment, or prompt.
func resolveSalt(flagSet bool, flagVal string) (string, error) {
	if flagSet {
		return flagVal, nil
	}
	if env, ok := os.LookupEnv(envSalt); ok {
		return env, nil
	}
	return readSecret("Salt (optional, press Enter for rclone's default): ")
}

// readSecret reads a line from the terminal without echoing it when stdin is a
// TTY, and falls back to a plain line read (so input can be piped, e.g. in
// tests or scripts) otherwise.
func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	line, err := stdin.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
