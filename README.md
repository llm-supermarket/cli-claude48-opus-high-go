# cli-claude48-opus-go
A small CLI tool that encrypts and decrypts using the rclone encryption defaults. 

Rclone uses a custom salt if no salt is provided, which this tool will use by default. A few similar tools:

- https://github.com/rclone/rclone
- https://github.com/mcolatosti/rclonedecrypt
- https://github.com/br0kenpixel/rclone-rcc
- @fyears/rclone-crypt

Rclone encryption uses: 
- NaCl SecretBox (XSalsa20 + Poly1305) for the file contents.
- AES256 for the filenames.
- scrypt for keymaterial.

Files produced by this tool are byte-for-byte compatible with rclone's `crypt`
backend, so they can be decrypted by rclone and vice versa.

## Installation

The CLI is a single self-contained binary with no runtime dependencies — no Go,
Python, or other framework installation required.

### Scoop (Windows)

```powershell
scoop bucket add cli-claude48-opus-high-go https://github.com/llm-supermarket/cli-claude48-opus-high-go
scoop install cli-claude48-opus-high-go
```

### Homebrew (macOS/Linux)

```bash
brew tap cli-claude48-opus-high-go https://github.com/llm-supermarket/cli-claude48-opus-high-go
brew install cli-claude48-opus-high-go
```

### From source

```bash
go install github.com/llm-supermarket/cli-claude48-opus-high-go@latest
```

## Usage

```text
cli-claude48-opus-high-go encrypt -i <input> [-o <output>] [flags]
cli-claude48-opus-high-go decrypt -i <input> [-o <output>] [flags]
cli-claude48-opus-high-go version
```

### Flags

| Flag | Description |
| --- | --- |
| `-i`, `--input-file` | Input file to encrypt or decrypt (**required**). |
| `-o`, `--output-file` | Output file (optional). When omitted, the filename itself is encrypted/decrypted automatically. |
| `--filename-encoding` | Encoding for the encrypted filename: `base32` (default, matches rclone) or `base64`. |
| `--password` | Password (⚠️ insecure — see the security note below). |
| `--salt` | Optional salt (rclone's "password2"). When omitted, rclone's default salt is used. |

If `--password` is not supplied, the tool reads it from the
`RCLONE_ENCRYPT_PASSWORD` environment variable, or prompts for it interactively
(without echoing). The optional salt can likewise be supplied via
`RCLONE_ENCRYPT_SALT` or entered at the prompt (press Enter for rclone's
default salt).

### Examples

Encrypt a file, getting prompted for the password and (optional) salt:

```bash
cli-claude48-opus-high-go encrypt -i secret.txt -o secret.enc
```

Decrypt it again:

```bash
cli-claude48-opus-high-go decrypt -i secret.enc -o secret.txt
```

Let the tool derive the output name by encrypting/decrypting the filename too
(rclone-style). Encrypting `TEST_FILE.txt` writes a file whose name is the
encrypted form:

```bash
cli-claude48-opus-high-go encrypt -i TEST_FILE.txt
# -> writes e.g. kr9tu4e1da4u3nifdd99g9tf5o
```

Decrypt by encrypted filename, restoring the original name:

```bash
cli-claude48-opus-high-go decrypt -i kr9tu4e1da4u3nifdd99g9tf5o
# -> writes TEST_FILE.txt
```

Use base64 filename encoding instead of the default base32:

```bash
cli-claude48-opus-high-go encrypt --filename-encoding base64 -i TEST_FILE.txt
cli-claude48-opus-high-go decrypt --filename-encoding base64 -i <encrypted-name>
```

Supply a custom salt:

```bash
cli-claude48-opus-high-go encrypt -i secret.txt -o secret.enc --salt "my-extra-salt"
```

Supply the password via an environment variable (recommended for scripting):

```bash
export RCLONE_ENCRYPT_PASSWORD='Testpassword1'
cli-claude48-opus-high-go encrypt -i secret.txt -o secret.enc
```

### Security note on `--password`

Passing `--password` on the command line is **insecure**:

- It is stored in your shell history.
- It is visible to other users via the process list (e.g. `ps`).

Prefer the interactive prompt or the `RCLONE_ENCRYPT_PASSWORD` environment
variable. If you must use `--password`, clear it from your shell history
afterwards:

```bash
# bash / zsh
history -d $(history 1 | awk '{print $1}')
```

```powershell
# PowerShell (PSReadLine)
Clear-History
# also remove it from the persisted history file:
Remove-Item (Get-PSReadLineOption).HistorySavePath
```

## Development

```bash
go build ./...     # build
go test ./...      # run the test suite
```

Releases are produced automatically by the
[`Build and Release`](.github/workflows/build-release.yml) GitHub Actions
workflow, which is triggered by pushing a `v*.*.*` tag. It cross-compiles
binaries for Windows, macOS, and Linux (amd64/arm64), publishes a GitHub
Release, and updates the Scoop manifest
(`cli-claude48-opus-high-go.json`) and Homebrew formula
(`Formula/cli-claude48-opus-high-go.rb`).

## License

[MIT](LICENSE)
