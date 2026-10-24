# cpc — Copy Context

快速搜索并复制开发上下文。Go 编写的单文件 CLI，支持 macOS 和 Linux，界面可切换中文或英文，并提前显示可复制的实际内容。

## Homebrew 安装（推荐）

macOS 和 Linux 用户可以通过 Homebrew 安装稳定版：

```sh
brew tap coffeedeveloper/cpc https://github.com/coffeedeveloper/cpc.git
brew install coffeedeveloper/cpc/cpc
```

安装后直接运行：

```sh
cpc --version
cpc
```

更新到最新稳定版：

```sh
brew update
brew upgrade coffeedeveloper/cpc/cpc
```

本仓库同时作为 Homebrew tap，首次添加需传入完整仓库 URL。Homebrew 会自动安装构建所需的 Go，校验源码 SHA-256，并编译安装 `cpc`，无需手动配置 Go 环境。

如需安装 main 分支的开发版，可使用 `brew install --HEAD coffeedeveloper/cpc/cpc`。已安装 HEAD 的用户切换稳定版时，先执行 `brew uninstall cpc`，再执行上述稳定版安装命令。

无需 Homebrew 的用户也可从 [GitHub Releases](https://github.com/coffeedeveloper/cpc/releases/latest) 下载 macOS/Linux 的 arm64/amd64 二进制，并使用同次发布的 `checksums.txt` 校验。

## 从源码构建

需要 Go 1.24 或更新版本：

```sh
make build
./bin/cpc
# 安装到 GOBIN，默认是 $(go env GOPATH)/bin；请将该目录加入 PATH
make install
```

## 命令

| 命令 | 复制内容 |
| --- | --- |
| `cpc` | 展示可搜索菜单，选择后复制 |
| `cpc ip` | 本地 IP；多地址时按网卡选择，IPv4 优先显示 |
| `cpc ip en0` | 指定网卡；该网卡有多个地址时继续选择 |
| `cpc public-ip` | 外网 IP |
| `cpc pwd` | 当前工作目录的绝对路径 |
| `cpc branch` | Git 当前分支名 |
| `cpc commit` | Git HEAD 的完整 commit hash |
| `cpc script` | 先选择任务，再选择要复制的内容 |
| `cpc script build` | 跳过任务列表，选择 build 的复制内容 |
| `cpc --print pwd` | 仅输出，不修改剪贴板 |
| `cpc --help` / `cpc --version` | 帮助 / 版本 |

直接输入中英文关键词即可过滤；多个关键词以空格分隔，忽略英文大小写。使用 **↓ / Ctrl+N** 向下选择、**↑ / Ctrl+P** 向上选择（macOS 同样支持）、**Enter** 确认、**Esc** 返回上一级（主菜单中取消）、**Ctrl+C** 取消。搜索覆盖命令名、中文名称、英文名称、简介及实际值；脚本列表还支持按脚本命令内容搜索。

例如在包含以下 `package.json` 的目录执行 `cpc script`，搜索并选中 `build` 后，会显示三种内容供选择；仅在选定内容后写入剪贴板，不会运行脚本：

```json
{"scripts": {"dev": "vite", "build": "vite build", "test": "vitest"}}
```

| 内容类型 | `build` 示例 | `--format` |
| --- | --- | --- |
| 脚本名字 | `build` | `name` |
| 执行命令 | `npm run build` | `command` |
| 脚本内容 | `vite build` | `content` |

内容列表支持搜索、方向键和 `Ctrl+N/P`，`Esc` 返回任务列表并恢复之前的搜索词与选中项。Node 执行命令自动识别 npm、pnpm、Yarn 或 Bun，生成 `<包管理器> run <脚本名>`；特殊字符会进行 shell 引用；脚本内容保留原始空白、换行及空字符串。

包管理器识别规则：从当前脚本所属的 package.json 目录向上查找，包含 Git 根目录，但不跨越它（没有 Git 仓库则查到文件系统根目录）。优先采用最近的 `packageManager` 声明，如 `"packageManager": "pnpm@10.0.0"`；否则采用最近一层的锁文件，因此 monorepo 子包也能继承根目录配置。子包自己的显式声明优先于根目录声明，祖先的显式声明优先于子包锁文件。

| 识别文件 | 执行命令示例 |
| --- | --- |
| `package-lock.json` / `npm-shrinkwrap.json` | `npm run build` |
| `pnpm-lock.yaml` | `pnpm run build` |
| `yarn.lock` | `yarn run build` |
| `bun.lock` / `bun.lockb` | `bun run build` |

没有任何识别信息时默认 npm。同层存在不同包管理器的锁文件且没有显式声明时会报错，提示在 package.json 设置 `packageManager`；不支持的声明或无效的 package.json 也会明确报错。识别不依赖安装包管理器，也不执行它们。任务名和原始脚本内容不受包管理器影响。

可以通过 `--format` 跳过内容选择。非交互使用需同时指定任务名和格式；`--print` 本身不会跳过选择：

```sh
cpc script build --format name                 # 复制 build
cpc script build --format command              # 复制 npm run build
cpc --print script build --format content      # 输出 vite build
```

`script` 从当前目录向上寻找最近包含 `package.json`、`go.mod` 或 `Cargo.toml` 的目录，因此在 `src/` 中也可使用。只使用最近的项目目录，不合并父项目。主菜单预览项目类型与任务数量。

| 项目 | 任务来源 | 三种内容 |
| --- | --- | --- |
| Node | `package.json` 的 `scripts` | 脚本名 / 对应包管理器的执行命令 / 原始脚本内容 |
| Go | 检测 `go.mod`，提供内置常用命令 | 任务名如 `test` / `go test ./...` / `go test ./...` |
| Rust | 检测 `Cargo.toml`（含 workspace），提供内置常用命令 | 任务名如 `build` / `cargo build` / `cargo build` |

Go/Rust 使用内置命令模板，因此“执行命令”和“脚本内容”相同，三项均保留。

Go 任务：`build` → `go build ./...`、`test` → `go test ./...`、`run` → `go run .`、`fmt` → `go fmt ./...`、`vet` → `go vet ./...`、`tidy` → `go mod tidy`、`generate` → `go generate ./...`。

Rust 任务：`build`、`check`、`test`、`run`、`fmt`、`clippy`、`clean` 对应同名 `cargo` 命令；`doc` 对应 `cargo doc --open`。这些是常用命令模板，默认在项目根目录使用；`run` 需要可运行的程序，`fmt`/`clippy` 需要对应 Rust 组件。不会推断项目自定义 Makefile 任务或 Cargo alias。

同一目录有多种项目时，列表和直接选择使用 `node:build`、`go:build`、`rust:build` 等前缀消除歧义，复制值不包含该前缀：

```sh
cpc script                       # 搜索选择
cpc --print script test --format command       # Go 项目输出 go test ./...
cpc --print script rust:build --format command # 混合项目输出 cargo build
```

读取和选择不需要安装 Node/Go/Rust 工具链，也不会执行命令。没有支持的项目、Node 脚本为空或 package.json 格式错误时显示明确错误。

命令参考：[Go command](https://pkg.go.dev/cmd/go)、[Cargo commands](https://doc.rust-lang.org/cargo/commands/index.html)。

本地 IP 排除未启动、loopback 和 link-local 地址，保留 VPN/虚拟网卡，选择后只复制地址。打开主菜单时后台访问 `https://api.ipify.org` 查询外网 IP，8 秒超时并验证返回地址；失败后可选中该项按 Enter 重试。直接执行其他命令不会查询外网 IP。Git 命令作用于当前工作目录；detached HEAD 没有分支名，会报错，但仍可复制 commit hash。尚无提交的仓库无法复制 commit hash。

macOS 使用系统 `pbcopy`。Linux 的 Wayland 使用 `wl-copy`（安装 `wl-clipboard`），X11 使用 `xclip` 或 `xsel`。无图形会话、SSH 或流水线可使用 `--print`。非交互环境需要明确命令，多个 IP 或脚本仍需交互选择时会报错。复制内容不带额外换行；`--print` 输出带换行。错误退出码为 1，取消为 130。

## 语言与预览

可手动创建 `~/.cpc.config.toml`：

```toml
language = "auto" # auto、zh 或 en
```

默认 `auto`，依次读取第一个非空的 `LC_ALL`、`LC_MESSAGES`、`LANG`：中文 locale 使用中文，其余（含未设置）使用英文。设置 `zh` 或 `en` 可覆盖系统语言。标题、帮助、说明、状态和应用错误提示只显示一种语言，命令名保持不变，中英文关键词都可搜索。无配置文件时正常运行；无效 TOML、不支持的语言或无法读取的配置会报错。程序不会自动写入配置文件。

主菜单立即显示工作目录，并异步加载 Git 分支、commit、本地 IP、脚本数量和外网 IP。commit 预览显示前 12 位，复制仍为完整哈希。多地址先显示首个 IP 和数量，选择后进入网卡列表；脚本列表展示任务名和命令内容，选择任务后继续选择脚本名、执行命令或脚本内容。加载中、非 Git 仓库、无提交等状态不会阻塞其他选项。

菜单在独立终端屏幕中显示，退出后恢复原屏幕。每行根据窗口宽度截断长内容，支持窗口缩放和列表滚动；底部显示当前选项的简介。缩略仅影响显示，复制值保持完整。菜单会话复用已经加载的数据，退出时取消未完成的网络请求。

## 发布维护

仓库已包含 CI、GoReleaser 配置和发布 workflow。推送新的版本 tag 会触发发布。

1. 将代码提交并推送至 `coffeedeveloper/cpc` 的 `main` 分支。
2. 确认 GitHub Actions 已启用，仓库允许 workflow 的 `contents: write` 权限。如果 main 有分支保护，需要允许发布机器人提交 Formula 更新，或手动执行下面的 Formula 更新步骤并走 PR。
3. 在通过 CI 的提交上创建版本 tag 并推送：

```sh
# 版本号需使用尚未发布的新版本，例如 v0.1.1
git tag -a v0.1.1 -m "Release v0.1.1"
git push origin v0.1.1
```

Release workflow 会执行测试、构建四个平台的二进制、创建 GitHub Release，并将稳定版源码 URL 与真实 SHA-256 更新到 `main` 的 `Formula/cpc.rb`。使用同仓库 `GITHUB_TOKEN`，无需跨仓库 PAT。后续版本重复推送新 tag 即可。预发布 tag（如 `v0.2.0-rc.1`）不会更新稳定 Formula；较旧版本也不会回退 Formula。

若 Release 成功但 Formula 推送受分支保护阻止，可在最新 main 上运行以下命令并提交变更：

```sh
python3 scripts/update_formula.py v0.1.0
```

不要移动已发布 tag。维护者可在发布前验证配置和离线打包（需要 GoReleaser v2 以及至少一个 Git 提交）：

```sh
make test
goreleaser check
goreleaser release --snapshot --clean
```

CI 在 macOS 和 Linux 上执行 race 测试、vet、构建以及配置校验。测试覆盖配置语言、终端宽度、菜单导航、异步预览、脚本查找/解析、Git 状态和公网 IP 响应验证。

可额外运行带终端模拟器的回归测试，检查 40/80/120 列、窗口缩放和连续选择后的屏幕内容：

```sh
make build
python3 -m venv /tmp/cpc-terminal-qa
/tmp/cpc-terminal-qa/bin/pip install pyte==0.8.2
/tmp/cpc-terminal-qa/bin/python scripts/test_terminal.py
```

参考：[Homebrew tap 文档](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap)、[GoReleaser GitHub Actions 文档](https://goreleaser.com/ci/actions/)。

## License

MIT
