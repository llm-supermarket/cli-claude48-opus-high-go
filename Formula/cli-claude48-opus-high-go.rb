class CliClaude48OpusHighGo < Formula
  desc "Encrypt and decrypt files using rclone's crypt defaults"
  homepage "https://github.com/llm-supermarket/cli-claude48-opus-high-go"
  version "0.0.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v0.0.0/cli-claude48-opus-high-go-darwin-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    else
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v0.0.0/cli-claude48-opus-high-go-darwin-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v0.0.0/cli-claude48-opus-high-go-linux-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    else
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v0.0.0/cli-claude48-opus-high-go-linux-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
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
