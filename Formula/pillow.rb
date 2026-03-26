class Pillow < Formula
  desc "Voice-narrated agentic coding with physical interrupts"
  homepage "https://github.com/AMOORCHING/pillow"
  url "https://github.com/AMOORCHING/pillow.git", tag: "v0.1.0"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"pillow", "./cmd/pillow"
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"pillowsensord", "./cmd/pillowsensord"
  end

  test do
    assert_match "Voice-narrated", shell_output("#{bin}/pillow --help")
  end
end
