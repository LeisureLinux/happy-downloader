# 欢喜下载 · Happy Downloader

一个自托管的 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 网页界面。单个静态二进制，不依赖运行时，不需要数据库服务——在局域网里任何一台机器的浏览器上打开它，视频就下载到有磁盘空间的那台机器上。

它是为一个具体的麻烦场景写的：一台没有浏览器、没有桌面会话的无头 Linux 主机，却要下载那些必须登录才能拿到的东西。Cookie 在另一台机器上刷新后推过来，服务端只负责盯着那个文件。

## 它能做什么

**在网页上下载。** 粘贴一个或多个链接，选好要什么，剩下的在服务端做。页面显示队列、实时进度，以及完整的 yt-dlp 日志。

**站点配置。** 每个站点一套参数，按主机名匹配（最长后缀优先，空主机是兜底）。在设置里改，不用重启。每次调用都带 `--ignore-config`，所以宿主机上那份 yt-dlp 配置不会偷偷改变下载行为——日志里看到的参数就是完整的参数。

**Cookie 文件，自动监听。** `paths.cookie_path` 既接受浏览器扩展导出的 JSON，也接受现成的 `cookies.txt`，格式从内容判断，转换时保留 `httpOnly`。文件被监听，替换即生效，不用重启。每次 yt-dlp 调用用的是它自己的临时副本——因为 `--cookies` 同时也会**回写**，共用一份文件会让一个进程截断另一个进程正在读的文件。

**PO Token 支持。** 把 `ytdlp.pot_base_url` 指向一个
[bgutil-ytdlp-pot-provider](https://github.com/Brainicism/bgutil-ytdlp-pot-provider)
实例，token 参数会自动拼好，连代理一起带上。

**是队列，不是历史。** 只列出没下完的。下完的文件不再算队列条目，而是出现在文件浏览里，并且会在完成时自动刷新。

**参数预览。** 设置页显示某个链接最终解析成的完整命令行，包含所有默认值。它和日志里记的是同一个字符串，所以失败可以照抄复现。

**带历史的日志面板。** 右下角的日志按钮打开一个面板，先显示最近 200 行，然后通过 SSE 实时流式追加。每条请求记录都带客户端地址，所以能看清局域网里是哪台机器要了什么。

**16 种语言**，包括简体中文和繁体中文，语言菜单里各自用本语言显示。

**主题。** 明暗两种，另有可选的主题色。

## 环境要求

- Linux（预编译二进制有 amd64 和 arm64；源码在 Go 支持的平台上都能编）
- `PATH` 里有 [yt-dlp](https://github.com/yt-dlp/yt-dlp)，或在配置里写绝对路径
- `ffmpeg`，如果要合并视频或转换音频
- 一个 JavaScript 运行时（`node` 20+）——没有它 YouTube 的挑战求解会失败，而且报错看起来像缺少格式，不像缺少运行时
- 一个 PO Token 提供方——YouTube 现在对大部分内容都要求它，安装步骤见
  [给 AI Agent 的安装说明](#给-ai-agent-的安装说明)

环境要求是最容易被低估的部分。如果你是给别人装，别只看这个列表，去读那一节：它按必须的先后顺序讲了四个外部组件，以及每装好一个该怎么验证。

## 安装

从 [Releases](https://github.com/LeisureLinux/happy-downloader/releases) 下载对应架构的二进制，放到一个稳定的位置。

```sh
mkdir -p ~/.local/share/happy-downloader/bin
curl -L -o ~/.local/share/happy-downloader/bin/happy-downloader \
  https://github.com/LeisureLinux/happy-downloader/releases/latest/download/happy-downloader_linux-arm64
chmod +x ~/.local/share/happy-downloader/bin/happy-downloader
```

写好配置文件然后启动：

```sh
~/.local/share/happy-downloader/bin/happy-downloader -conf ~/.config/happy-downloader/config.yml
```

服务默认监听 `0.0.0.0:3033`，前端由同一个二进制提供。

### 用用户级服务常驻

用用户单元而不是系统单元，因为读取浏览器的 cookie 数据库需要登录密钥环，而密钥环只存在于会话总线里。

`~/.config/systemd/user/happy-downloader.service`：

```ini
[Unit]
Description=Happy Downloader
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h
ExecStart=%h/.local/share/happy-downloader/bin/happy-downloader -conf %h/.config/happy-downloader/config.yml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now happy-downloader.service
loginctl enable-linger "$USER"   # 退出登录后继续运行
```

### 反向代理

应用自身没有 TLS，也没有登录界面。把它放在 nginx 后面，留在局域网内。现成的配置在
[`packaging/nginx/happydl.conf`](packaging/nginx/happydl.conf)；要紧的是 WebSocket 升级头、日志流用的 `proxy_buffering off`，以及大文件需要的长超时。

如果挂在子目录下，设置 `server.base_url`。

### 从另一台机器刷新 Cookie

无头主机没有浏览器，cookie 只能从别处来。
[`scripts/refresh-cookies.sh`](scripts/refresh-cookies.sh) 读取浏览器配置，按站点过滤，拒绝写出游客会话，并且可以一步推到服务器：

```sh
scripts/refresh-cookies.sh \
  --output ~/.config/happy-downloader/youtube-cookie.json \
  --browser chrome+gnomekeyring \
  --deploy user@server:/home/user/.config/happy-downloader/youtube-cookie.json
```

配套的 systemd 定时器在 [`packaging/systemd/`](packaging/systemd/) 里，那里的说明解释了密钥环的依赖。

## 给 AI Agent 的安装说明

> **如果你是 Agent，正在给用户的机器装这个东西，先把这一整节读完再动手。**
> 这一节是照着做就能完成的：每条命令都是完整的，版本是互相测试过的，顺序不能换。
> 欢喜下载本身是最小的一部分——一个静态二进制。真正要做的是它调用的那四样东西。

### 必须先有的东西

| 组件 | 作用 | 是否必需 |
|---|---|---|
| `yt-dlp` | 真正干活下载 | 必需 |
| JavaScript 运行时（`node`） | 解 YouTube 的挑战（`n` 参数）；没有它 YouTube 只返回图片，或者什么都不返回 | YouTube 必需 |
| `ffmpeg` | 合并分离的视频流和音频流；转换音频格式 | 必需，除非每次都只下单一流 |
| PO Token 提供方 | 证明这个会话不是机器人。YouTube 现在对大部分内容都要求它，没有就会报「Sign in to confirm you're not a bot」 | YouTube 必需 |
| Cookie 导出文件 | 携带登录状态。年龄限制、会员、地区限制的内容必需，普通视频现在也越来越多需要 | 实际上必需 |
| 欢喜下载 | 网页界面 | 必需 |

`yt-dlp` 大约每月发一版，YouTube 也经常把旧版本搞坏。把它装在**不用重新编译本项目就能更新**的地方——二进制从配置里读路径，不捆绑它。

### 第 1 步 —— 系统软件包

Debian/Ubuntu。换成你面前的发行版对应的写法；`node` 必须 20 或更新，`ffmpeg` 近几年的版本都行。

```sh
sudo apt update
sudo apt install -y ffmpeg python3 python3-venv curl
# Node 20+ —— 发行版自带的 nodejs 可能太旧
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
node --version    # 必须打印 v20 或更新
```

### 第 2 步 —— yt-dlp 装在独立 venv 里

不要 `apt install yt-dlp`，也不要 `pip install --user`：发行版包会落后好几个月，而全局 pip 安装会和发行版自己的 Python 包打架。venv 是可以随时丢掉重来的。

```sh
python3 -m venv ~/.local/share/happy-downloader/venv
~/.local/share/happy-downloader/venv/bin/pip install -U pip
~/.local/share/happy-downloader/venv/bin/pip install -U yt-dlp
~/.local/share/happy-downloader/venv/bin/yt-dlp --version
```

以后更新就是在同一个 venv 里 `pip install -U yt-dlp`。**下载开始大面积失败时，先做这个**——大部分问题当场解决。

### 第 3 步 —— PO Token 提供方

这一步是最容易装错的，也是缺了之后报错最让人摸不着头脑的。`bgutil-ytdlp-pot-provider` 分两半：一个 **yt-dlp 插件**（装进 venv，让 yt-dlp 知道怎么去要），一个**服务端**（一个 Node 进程，真正生成 token）。

```sh
# 插件，装进和 yt-dlp 同一个 venv
~/.local/share/happy-downloader/venv/bin/pip install -U bgutil-ytdlp-pot-provider

# 服务端
cd ~/.local/share/happy-downloader
git clone --depth 1 https://github.com/Brainicism/bgutil-ytdlp-pot-provider.git
cd bgutil-ytdlp-pot-provider/server
npm install
npx tsc          # 产出 build/main.js
```

接上去之前先确认它能用。每个 token 要几秒：

```sh
node build/main.js --host 127.0.0.1 --port 4416 &
curl -s -X POST http://127.0.0.1:4416/get_pot \
  -H 'Content-Type: application/json' \
  -d '{"content_binding":"dQw4w9WgXcQ"}' | head -c 200
```

返回带 `poToken` 字段的 JSON 就是好的。空响应或超时说明它到不了 YouTube——看下面关于代理的说明。

做成服务，开机自启。`~/.config/systemd/user/bgutil-pot.service`：

```ini
[Unit]
Description=bgutil PO Token provider
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%h/.local/share/happy-downloader/bgutil-ytdlp-pot-provider/server
ExecStart=/usr/bin/node build/main.js --host 127.0.0.1 --port 4416
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now bgutil-pot.service
```

### 第 4 步 —— Cookie 导出

服务端没有浏览器。它不能登录，也不能从浏览器配置里提取 cookie。导出必须在**有浏览器的机器**上做，然后拷过去。

**在有浏览器的机器上导出。** 要么用 cookie 导出扩展（"Get cookies.txt LOCALLY" 这一系——确认是本地的那个版本，它不上传任何东西），存成 JSON 或 Netscape 格式；要么用本仓库的脚本，直接读浏览器配置：

```sh
scripts/refresh-cookies.sh --output ~/youtube-cookie.json --browser chrome
```

脚本需要登录密钥环，所以只能在桌面会话里跑。这也是为什么它是**用户**单元而不是系统单元。

**拷到服务器**，然后把 `paths.cookie_path` 指向它：

```sh
scp ~/youtube-cookie.json user@server:/home/user/.config/happy-downloader/youtube-cookie.json
```

服务端会监听这个文件。替换它就能刷新会话，不用重启——所以导出是推过去的，而不是配置一次就完。

两个值得知道的失败模式：

- **游客导出里也仍然有 `LOGIN_INFO`**，所以「文件里有 cookie」不等于「有登录会话」。用
  `~/.local/share/happy-downloader/venv/bin/yt-dlp --cookies <文件> --print title <链接>`
  验证，看标题能不能正常解析。
- **只写 `chrome` 时，yt-dlp 认不出密钥环后端会静默解不开 v11 cookie。** 显式写
  `chrome+gnomekeyring`（或 `+kwallet`）。导出文件看起来一切正常——但它大概只有
  170 条 cookie，而不是几千条。

### 第 5 步 —— 欢喜下载本体

```sh
mkdir -p ~/.local/share/happy-downloader/bin ~/.config/happy-downloader
curl -L -o ~/.local/share/happy-downloader/bin/happy-downloader \
  https://github.com/LeisureLinux/happy-downloader/releases/latest/download/happy-downloader_linux-amd64
chmod +x ~/.local/share/happy-downloader/bin/happy-downloader
```

树莓派或其他 ARM 主机用 `linux-arm64`。然后写
`~/.config/happy-downloader/config.yml`——下面每条路径都是绝对路径，因为用户服务是从家目录启动的，相对路径会落到意想不到的地方：

```yaml
server:
  host: 127.0.0.1
  port: 3033

paths:
  download_path: /home/USER/media/downloads
  downloader_path: /home/USER/.local/share/happy-downloader/venv/bin/yt-dlp
  local_database_path: /home/USER/.local/share/happy-downloader/data
  js_runtime_path: /usr/bin/node
  cookie_path: /home/USER/.config/happy-downloader/youtube-cookie.json

ytdlp:
  pot_base_url: http://127.0.0.1:4416
  # proxy: http://127.0.0.1:8888   # 只有主机需要代理才能访问 YouTube 时才填
```

按[用用户级服务常驻](#用用户级服务常驻)那一节装好单元并启动。注意
`loginctl enable-linger`——不做这一步，最后一个会话关闭时服务就停了，看起来像崩溃。

### 第 6 步 —— 验证

最有用的测法是**整行一次跑完**，因为服务端跑的就是这一行：

```sh
~/.local/share/happy-downloader/venv/bin/yt-dlp \
  --cookies ~/.config/happy-downloader/youtube-cookie.json \
  --js-runtimes node:/usr/bin/node \
  --extractor-args 'youtubepot-bgutilhttp:base_url=http://127.0.0.1:4416' \
  --remote-components ejs:github \
  --print '%(title)s | %(ext)s' 'https://youtu.be/dQw4w9WgXcQ'
```

打印出标题和格式，就说明四个组件串起来了。

**不要把它缩短成一次只测一样。** 没有 PO Token 提供方时，yt-dlp 现在对大部分
YouTube 视频会报 `Only images are available` 或 `Requested format is not available`
——这两个报错里一个缺失的组件都没提到。所以一条不完整的命令在**装好的机器上也会失败**，
把你引去找一个并不存在的故障。整行跑的那个测法只有在全都装好时才通过，而这正是要问的问题。

然后确认服务端看到的是同一件事：

```sh
# 服务起来了，并且找得到 yt-dlp
curl -s http://127.0.0.1:3033/api/v1/version

# 端到端，走应用
curl -s -X POST http://127.0.0.1:3033/rpc/http \
  -H 'Content-Type: application/json' \
  -d '{"method":"Service.Exec","params":[{"url":"https://youtu.be/dQw4w9WgXcQ","params":[]}],"id":1}'
```

最后一个调用会返回一个 id。打开网页，点顶栏的日志按钮，确认 `msg="yt-dlp exec"`
那一行带的是完整参数。那一行是服务端所有行为的准绳——如果下载的表现和上面那条命令不一样，
差别就在那一行里。

### 装不上时

错误信息很具体，每条对应一个原因：

| 信息 | 原因 |
|---|---|
| `Sign in to confirm you're not a bot` | 没有 cookie 文件，或导出的是游客会话，或会话过期。重新导出。 |
| `Requested format is not available` / `Only images are available` | 没有 PO Token 提供方，或缺少 JavaScript 运行时。检查第 2、3 步。 |
| `n challenge solving failed` | JavaScript 运行时缺失或太旧。`node --version` 必须 20+。 |
| 字幕 `HTTP Error 429` | YouTube 在限流字幕。服务端带了 `--ignore-errors`，视频照样下；这是预期行为。 |
| 什么都超时 | 主机到不了 YouTube。确认是否需要代理，设置 `ytdlp.proxy`。 |
| 退出登录后服务就停 | 没跑 `loginctl enable-linger`。 |

这些问题在日志面板里都能看到，连完整命令行一起——这正是它们可诊断而不是神秘的原因。

### 如果主机需要代理

在 YouTube 直连不通的网络上——DNS 被污染、IP 段被封——设置 `ytdlp.proxy`，
服务端会在每次调用时带上 `--proxy`。PO Token 提供方需要同样的出口，但它自己没有
`--proxy` 参数；服务端会把代理写进它的 extractor-args 值里，所以设置一次两边都管。

先确认再假设：如果 `curl -sI https://www.youtube.com` 返回 `000` 或者连接被重置，
那就是直连不可用，代理不是可选项。

## 配置

`-conf` 指定 YAML 配置文件路径，默认 `./config.yml`。

```yaml
server:
  host: 0.0.0.0          # 默认值；放在反向代理后面就改成 127.0.0.1
  port: 3033
  base_url: ""           # 挂在子目录下时填写
  queue_size: 4          # 同时下载数；默认等于 CPU 核数

paths:
  download_path: /srv/media/downloads
  downloader_path: yt-dlp          # 也可以写绝对路径
  local_database_path: /var/lib/happy-downloader
  js_runtime_path: /usr/bin/node   # 用于 YouTube 挑战求解
  cookie_path: /etc/happy-downloader/youtube-cookie.json

ytdlp:
  pot_base_url: http://127.0.0.1:4416   # bgutil PO Token 提供方
  proxy: http://127.0.0.1:8888          # 出口代理，如果主机需要

logging:
  log_path: /var/log/happy-downloader.log
  enable_file_logging: false
```

相对路径按配置文件所在目录解析。

每个键也可以用环境变量设置，前缀 `APP_`，点号换成下划线——`APP_SERVER_PORT=8080`、
`APP_PATHS_COOKIE_PATH=/etc/cookies.json`。这样密钥不必写进配置文件。

站点级覆盖（不同的代理、不同的 cookie 文件、额外参数）在设置页里配置，存在本地数据库，不在这里。

### 访问控制

没有。登录页、JWT 中间件和 OpenID 集成都移除了：这个项目是给家庭网络用的，一套容易忘记开启、又难配置的登录机制，比没有更糟。用反向代理把它挡在公网之外，需要认证就在那一层做。

## 自行编译

```sh
cd frontend && pnpm install && pnpm build && cd ..

CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerVersion=$(git describe --tags --always) \
    -X github.com/LeisureLinux/happy-downloader/v4/server/rest.ServerBuiltAt=$(date +%s)" \
  -o happy-downloader main.go
```

前端通过 `go:embed` 编进二进制，所以顺序有讲究：先构建前端。

需要 Go 1.26+ 和带 pnpm 的 Node。

## HTTP 接口

前端通过 JSON-RPC 和后端通信：`POST /rpc/http`（实时状态走 `/rpc/ws`）。另有几个普通端点：

| 端点 | 用途 |
|---|---|
| `GET /api/v1/version` | 服务端和 yt-dlp 版本 |
| `GET /api/v1/cookies/status` | Cookie 文件状态 |
| `GET /api/v1/profiles` | 站点配置 |
| `POST /filebrowser/downloaded` | 列出已下载文件 |
| `GET /filebrowser/d/{id}` | 下载文件 |
| `GET /filebrowser/v/{id}` | 流式播放文件 |
| `GET /log/sse`、`/log/ws` | 实时日志流 |
| `GET /status` | 队列和连接状态 |

探测链接可用格式是 JSON-RPC 方法 `Service.Formats`，和其他调用一样走 `/rpc/http`。

完整 OpenAPI 文档在 `/openapi`。

## 许可

GPL-3.0-only。见 [LICENSE](LICENSE)。

本项目是 [marcopeocchi/yt-dlp-web-ui](https://github.com/marcopeocchi/yt-dlp-web-ui) 的分支，上游同样是 GPL-3.0。原始的版权声明和许可条款完整保留；本分支的修改以相同条款发布。

yt-dlp 自身是 Unlicense；yt-dlp 调用的 ffmpeg 有自己的许可条款，重新分发下载内容前请自行确认。

## English

[README.md](README.md)
