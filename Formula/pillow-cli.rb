class PillowCli < Formula
  desc "Voice-narrated agentic coding with physical interrupts"
  homepage "https://github.com/AMOORCHING/pillow"
  url "https://github.com/AMOORCHING/pillow.git", tag: "v0.1.0"
  license "MIT"

  depends_on "go" => :build

  conflicts_with "pillow", because: "both install a pillow binary"

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"pillow", "./cmd/pillow"
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"pillowsensord", "./cmd/pillowsensord"
  end

  service do
    run [opt_bin/"pillowsensord"]
    require_root true
    keep_alive true
    log_path var/"log/pillowsensord.log"
    error_log_path var/"log/pillowsensord.log"
  end

  test do
    assert_match "Voice-narrated", shell_output("#{bin}/pillow --help")
  end
end
