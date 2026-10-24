class Cpc < Formula
  desc "Search and copy development context to your clipboard"
  homepage "https://github.com/coffeedeveloper/cpc"
  # Stable URL and checksum are added automatically by the release workflow.
  license "MIT"
  head "https://github.com/coffeedeveloper/cpc.git", branch: "main"

  depends_on "go" => :build

  def install
    build_version = build.head? ? "HEAD" : version.to_s
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{build_version}"), "./cmd/cpc"
  end

  test do
    assert_match "cpc", shell_output("#{bin}/cpc --version")
    assert_equal testpath.realpath.to_s, shell_output("#{bin}/cpc --print pwd").strip
  end
end
