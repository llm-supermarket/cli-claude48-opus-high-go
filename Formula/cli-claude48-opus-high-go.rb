class CliClaude48OpusHighGo < Formula
  desc "Encrypt and decrypt files using rclone's crypt defaults"
  homepage "https://github.com/llm-supermarket/cli-claude48-opus-high-go"
  version "1.0.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v1.0.0/cli-claude48-opus-high-go-darwin-arm64.tar.gz"
      sha256 "b49f3f3e6bbdf36f94e5c7b25f7efb755a91e16b4b89902ce2d334e2d2092dc9"
    else
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v1.0.0/cli-claude48-opus-high-go-darwin-amd64.tar.gz"
      sha256 "456a1764ea4280a3dc4be0d89f79a5242c6f0a41e7ba2153403a479da94322aa"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v1.0.0/cli-claude48-opus-high-go-linux-arm64.tar.gz"
      sha256 "a767447b334d320ca09d4e5d46dadb6d3e89748f3884c1ea1964e679a86bd7b3"
    else
      url "https://github.com/llm-supermarket/cli-claude48-opus-high-go/releases/download/v1.0.0/cli-claude48-opus-high-go-linux-amd64.tar.gz"
      sha256 "1815108a3d244027b3bf18cc869ef48b173d6d0609ad77d65e8efbe2662bbc85"
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