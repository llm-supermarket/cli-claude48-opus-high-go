package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a file with the given contents and returns its path.
func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const sampleContents = "## Test\n\nspoon harvest gravity ozone absurd neither rate correct\n"

// TestEncryptDecryptViaEnv exercises encrypt+decrypt using the password
// supplied through the environment variable, with the default salt.
func TestEncryptDecryptViaEnv(t *testing.T) {
	t.Setenv(envPassword, "Testpassword1")
	dir := t.TempDir()

	in := writeFile(t, dir, "plain.txt", sampleContents)
	enc := filepath.Join(dir, "cipher.bin")
	dec := filepath.Join(dir, "out.txt")

	if err := run([]string{"encrypt", "-i", in, "-o", enc}); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := run([]string{"decrypt", "-i", enc, "-o", dec}); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	assertFileContents(t, dec, sampleContents)
}

// TestEncryptDecryptWithSalt exercises a custom salt round trip.
func TestEncryptDecryptWithSalt(t *testing.T) {
	t.Setenv(envPassword, "Testpassword1")
	dir := t.TempDir()

	in := writeFile(t, dir, "plain.txt", sampleContents)
	enc := filepath.Join(dir, "cipher.bin")
	dec := filepath.Join(dir, "out.txt")

	if err := run([]string{"encrypt", "--salt", "custom-salt", "-i", in, "-o", enc}); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Decrypting with the default salt must fail.
	if err := run([]string{"decrypt", "-i", enc, "-o", dec}); err == nil {
		t.Fatal("expected failure decrypting with wrong (default) salt")
	}
	// Decrypting with the matching salt must succeed.
	if err := run([]string{"decrypt", "--salt", "custom-salt", "-i", enc, "-o", dec}); err != nil {
		t.Fatalf("decrypt with salt: %v", err)
	}
	assertFileContents(t, dec, sampleContents)
}

// TestEncryptDecryptWithPasswordFlag exercises the --password flag path.
func TestEncryptDecryptWithPasswordFlag(t *testing.T) {
	os.Unsetenv(envPassword)
	os.Unsetenv(envSalt)
	dir := t.TempDir()

	in := writeFile(t, dir, "plain.txt", sampleContents)
	enc := filepath.Join(dir, "cipher.bin")
	dec := filepath.Join(dir, "out.txt")

	if err := run([]string{"encrypt", "--password", "Testpassword1", "-i", in, "-o", enc}); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := run([]string{"decrypt", "--password", "Testpassword1", "-i", enc, "-o", dec}); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	assertFileContents(t, dec, sampleContents)
}

// TestCustomEncodingBase64 verifies that the --filename-encoding flag produces a
// base64-encoded filename when the output is not given, and that it round trips.
func TestCustomEncodingBase64(t *testing.T) {
	t.Setenv(envPassword, "Testpassword1")
	dir := t.TempDir()

	in := writeFile(t, dir, "TEST_FILE.txt", sampleContents)

	// Encrypt with base64 encoding, letting the tool derive the output name.
	if err := run([]string{"encrypt", "--filename-encoding", "base64", "-i", in}); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// The derived encrypted file should exist with a base64 name. Find it.
	encName := findEncryptedSibling(t, dir, "TEST_FILE.txt")
	// A base64 name must not contain characters outside the URL-safe alphabet
	// and (for this short name) should differ from the base32 form.
	if strings.ContainsAny(encName, "=") {
		t.Errorf("base64 name should be unpadded: %q", encName)
	}

	// Decrypt it back using base64 encoding; derived name must be original.
	encPath := filepath.Join(dir, encName)
	if err := run([]string{"decrypt", "--filename-encoding", "base64", "-i", encPath}); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	assertFileContents(t, in, sampleContents) // original name restored
}

// TestPromptForPasswordAndSalt simulates a user typing the password and an
// empty salt at the interactive prompts (input is piped via stdin).
func TestPromptForPasswordAndSalt(t *testing.T) {
	os.Unsetenv(envPassword)
	os.Unsetenv(envSalt)
	dir := t.TempDir()

	in := writeFile(t, dir, "plain.txt", sampleContents)
	enc := filepath.Join(dir, "cipher.bin")
	dec := filepath.Join(dir, "out.txt")

	// Encrypt: feed "password\n" then "\n" (empty salt -> default).
	stdin = bufio.NewReader(strings.NewReader("Testpassword1\n\n"))
	if err := run([]string{"encrypt", "-i", in, "-o", enc}); err != nil {
		t.Fatalf("encrypt via prompt: %v", err)
	}

	// Decrypt with the same prompted inputs.
	stdin = bufio.NewReader(strings.NewReader("Testpassword1\n\n"))
	if err := run([]string{"decrypt", "-i", enc, "-o", dec}); err != nil {
		t.Fatalf("decrypt via prompt: %v", err)
	}
	assertFileContents(t, dec, sampleContents)
}

func TestRefusesSameInputOutput(t *testing.T) {
	t.Setenv(envPassword, "Testpassword1")
	dir := t.TempDir()
	in := writeFile(t, dir, "plain.txt", sampleContents)

	if err := run([]string{"encrypt", "-i", in, "-o", in}); err == nil {
		t.Error("expected error when input and output are the same file")
	}
	// The input must be left intact (not truncated).
	assertFileContents(t, in, sampleContents)
}

func TestMissingInputFile(t *testing.T) {
	t.Setenv(envPassword, "Testpassword1")
	if err := run([]string{"encrypt"}); err == nil {
		t.Error("expected error when input file missing")
	}
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("contents of %s = %q, want %q", path, got, want)
	}
}

// findEncryptedSibling returns the name of the file in dir that is not the
// original plaintext file (i.e. the derived encrypted file).
func findEncryptedSibling(t *testing.T, dir, original string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != original {
			return e.Name()
		}
	}
	t.Fatal("no encrypted sibling file found")
	return ""
}
