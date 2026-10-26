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
- 一个 JavaScript 运行时（`node` 或 `deno`），用于 YouTube 的挑战求解
- 可选：PO Token 提供方，如果 YouTube 要求你证明自己不是机器人

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
