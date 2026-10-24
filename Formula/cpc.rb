class Cpc < Formula
  desc "Search and copy development context to your clipboard"
  homepage "https://github.com/coffeedeveloper/cpc"
  url "https://github.com/coffeedeveloper/cpc/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "c930042f8f9417254a318dd3c2e78f0a3738470120a702033185e92c0a20ca04"
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
