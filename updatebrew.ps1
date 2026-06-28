param(
    [Parameter(Mandatory = $true)]
    [string]$Version
)

$repo = "llm-supermarket/cli-claude48-opus-high-go"
$platforms = @("darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64")
$formulaPath = "$PSScriptRoot/Formula/cli-claude48-opus-high-go.rb"
$base = "https://github.com/$repo/releases/download/v$Version"

# Download each release tarball and compute its SHA256.
$hash = @{}
foreach ($platform in $platforms) {
    $url = "$base/cli-claude48-opus-high-go-$platform.tar.gz"
    $tempFile = Join-Path ([System.IO.Path]::GetTempPath()) "cli-claude48-opus-high-go-$platform.tar.gz"

    Write-Host "Downloading $url ..."
    Invoke-WebRequest -Uri $url -OutFile $tempFile

    $hash[$platform] = (Get-FileHash -Path $tempFile -Algorithm SHA256).Hash.ToLower()
    Write-Host "SHA256 for ${platform}: $($hash[$platform])"

    Remove-Item $tempFile
}

# Regenerate the formula wholesale so it stays correct across releases (a
# placeholder-substitution approach only works for the first release, and a
# case-insensitive replace would clobber the `version` keyword / `--version`).
$formula = @"
class CliClaude48OpusHighGo < Formula
  desc "Encrypt and decrypt files using rclone's crypt defaults"
  homepage "https://github.com/$repo"
  version "$Version"

  on_macos do
    if Hardware::CPU.arm?
      url "$base/cli-claude48-opus-high-go-darwin-arm64.tar.gz"
      sha256 "$($hash['darwin-arm64'])"
    else
      url "$base/cli-claude48-opus-high-go-darwin-amd64.tar.gz"
      sha256 "$($hash['darwin-amd64'])"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "$base/cli-claude48-opus-high-go-linux-arm64.tar.gz"
      sha256 "$($hash['linux-arm64'])"
    else
      url "$base/cli-claude48-opus-high-go-linux-amd64.tar.gz"
      sha256 "$($hash['linux-amd64'])"
    end
  end

  def install
    bin.install "cli-claude48-opus-high-go-darwin-arm64" => "cli-claude48-opus-high-go" if OS.mac? && Hardware::CPU.arm?
    bin.install "cli-claude48-opus-high-go-darwin-amd64" => "cli-claude48-opus-high-go" if OS.mac? && !Hardware::CPU.arm?
    bin.install "cli-claude48-opus-high-go-linux-arm64" => "cli-claude48-opus-high-go" if OS.linux? && Hardware::CPU.arm?
    bin.install "cli-claude48-opus-high-go-linux-amd64" => "cli-claude48-opus-high-go" if OS.linux? && !Hardware::CPU.arm?
  end

  test do
    assert_match "cli-claude48-opus-high-go #{version}", shell_output("#{bin}/cli-claude48-opus-high-go --version")
  end
end
"@

Set-Content -Path $formulaPath -Value $formula -NoNewline
Write-Host "Wrote $formulaPath for version $Version"
