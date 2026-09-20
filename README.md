<p align="center">
    <h3 align="center">SmartPingNext | 开源、高效、便捷的网络质量监控神器</h3>
    <p align="center">
       一个综合性网络质量(PING)检测工具，支持正/反向PING绘图、互PING拓扑绘图与报警、全国PING延迟地图与在线检测工具等功能
        <br>
        <a href="./README_EN.md">English README</a>
        <br>
        <br>
        <a href="https://github.com/Antman2023/SmartPingNext/releases">
            <img src="https://img.shields.io/github/release/Antman2023/SmartPingNext.svg" >
        </a>
        <a href="https://github.com/Antman2023/SmartPingNext/blob/master/LICENSE">
            <img src="https://img.shields.io/hexpm/l/plug.svg" >
        </a>
    </p>
</p>

## 界面展示

![界面展示](./assets/界面展示.png)

## 功能

- 正向PING，反向Ping绘图
- 互PING间机器的状态拓扑，自定义延迟、丢包阈值报警（声音报警），报警时MTR检测
- 全国PING延迟地图（各省份可分电信、联通、移动三条线路）
- 检测工具，支持使用SmartPingNext各节点进行网络相关检测
- 节点编辑（支持修改节点名称和IP地址，自动同步关联配置）
- 配置导入/导出
- 支持深色/浅色主题切换
- 设置面板统一管理主题与语言
- 中英双语界面，支持运行时切换（无需刷新）
- 可收缩侧边栏
- 单文件部署，无需额外配置文件

## 技术栈

- **后端**: Go 1.24 + SQLite3（纯 Go 实现，无 CGO 依赖）
- **前端**: Vue 3 + TypeScript + Vite + Element Plus + ECharts

## 快速开始

### 下载发布版

从 [Releases](https://github.com/Antman2023/SmartPingNext/releases) 下载对应平台的版本：

| 平台 | 架构 | 文件 |
|------|------|------|
| Linux | amd64 | smartping-linux-amd64.tar.gz |
| Linux | arm64 | smartping-linux-arm64.tar.gz |
| Linux | armv7 | smartping-linux-armv7.tar.gz |
| Windows | amd64 | smartping-windows-amd64.zip |
| macOS | arm64 | smartping-darwin-arm64.tar.gz |

```bash
# Linux/macOS
tar -xzf smartping-*.tar.gz
./smartping

# Windows
# 解压 smartping-*.zip
# 双击运行 smartping.exe
```

首次运行会自动创建 `conf/`、`db/`、`logs/` 目录并释放默认配置文件。

文件日志按级别写入 `info.log`、`debug.log` 和 `error.log`。每个文件达到 10 MiB 后在下次写入前轮转，最多保留 3 个备份（`.1` 最新、`.3` 最旧），更早的日志自动删除；重启后继续按现有文件大小轮转。单条超大日志会完整保留，因此单个文件可能超过 10 MiB。默认日志级别为 `info`，可通过 `SMARTPING_LOG_LEVEL` 调整。标准输出仍保留，由 Docker 等运行环境管理其日志留存。

### 从源码构建

需要 Go 1.24+，以及 Node.js 20.19+ 或 22.12+。

```bash
# 前端
cd web
npm ci
npm run build:embed

# 后端
cd ..
go build -o smartping src/smartping.go
```

`build:embed` 支持 Windows、Linux 和 macOS，会在前端构建成功后替换 `src/static/html` 中的生成文件，避免重复复制产生嵌套目录或打包旧资源。仅开发前端时仍可使用 `npm run build`。

### 本地开发与验证

完成上述前端构建后，启动上一步构建的程序（Linux/macOS 使用 `./smartping`；Windows 可先执行 `go build -o smartping.exe ./src`，再运行 `.\smartping.exe`），再打开另一个终端进入 `web` 并运行 `npm run dev`。浏览器访问终端显示的开发地址（默认端口 3000），API 请求会代理到 `http://localhost:8899`。

运行时配置、数据库和日志存放在可执行文件旁的目录中。为保留开发数据，使用固定位置的构建产物，而非 `go run` 的临时程序。

可在 `web/.env.development.local` 中覆盖以下设置，修改后重启开发服务：

| 变量 | 默认值 | 用途 |
|------|--------|------|
| `VITE_PROXY_TARGET` | `http://localhost:8899` | 开发代理的后端地址 |
| `VITE_API_BASE_URL` | `/api` | 浏览器请求的 API 前缀，包含配置和密码验证接口 |
| `VITE_API_TIMEOUT` | `15000` | 普通 API 请求超时，单位为毫秒；节点代理请求会留出额外响应时间 |
| `VITE_DEFAULT_TIME_RANGE` | `6` | 正向与反向详情的默认小时数，允许 1 分钟至 31 天；无效值回退到 6 小时 |

使用开发代理时保留 `/api` 前缀，只修改 `VITE_PROXY_TARGET`。代理保留浏览器的 `Host` 和 `Origin`，以兼容后端对配置操作的来源检查。`VITE_` 变量会进入前端代码，不能用于保存密码；生产环境修改这些变量后需要重新构建前端及 Go 程序。

```bash
# web 目录：测试与静态检查（包括本地代理集成测试）
npm test
npm run lint

# 项目根目录：后端测试与静态检查
go test ./src/...
go vet ./src/...
```

配置导入仅接受不超过 16 MiB 的文件，导入后仍需保存才会应用到节点。导入、导出密码验证失败或超时不会丢弃当前编辑内容。

在线检测、定时 Ping、地图探测及告警 MTR 的域名解析最多等待 5 秒；调用方取消或更短的截止时间会提前结束解析。直接填写 IPv4 地址会跳过 DNS。

Windows 上可使用 Zig 运行 Go 数据竞争检测：安装 Go 和 Zig 并加入 PATH 后，在项目根目录执行 `pwsh -File scripts/test-race-windows.ps1`。脚本使用 `zig cc`，为 race 测试补充 Windows 同步库和固定加载方式，并在退出时恢复环境变量；正式构建仍不依赖 CGO。此流程已在 Go 1.27.1、Zig 0.16.0、Windows amd64 上验证。

资源同步会先完整复制到临时目录，再替换旧版。复制失败会保留旧版，替换失败会尝试恢复旧版；如果恢复也失败，命令会报告保留的备份路径，便于手动恢复。

部署到子路径（例如 `/smartping/`）时，先在 `web/.env.local` 设置 `VITE_API_BASE_URL=/smartping/api`，然后在 `web/` 下运行：

```bash
npm run build -- --base=/smartping/
node scripts/sync-embed.mjs
```

随后重新构建后端。反向代理需要将 `/smartping/` 下的请求转发给 SmartPing，并去掉该前缀（例如 `/smartping/api/config.json` 转发为 `/api/config.json`）。前端路由与静态资源会使用构建时指定的基路径。

### Docker

支持多架构镜像：`linux/amd64`、`linux/arm64`、`linux/arm/v7`

```bash
docker pull pathletboy/smartping-next:latest

# 运行容器
docker run -d \
  --name smartping \
  -p 8899:8899 \
  -v smartping-conf:/app/conf \
  -v smartping-db:/app/db \
  -v smartping-logs:/app/logs \
  --restart unless-stopped \
  pathletboy/smartping-next:latest

# 或自行构建
docker build -t smartping-next:latest .
docker run -d --name smartping -p 8899:8899 smartping-next:latest

# 或使用 docker-compose
docker-compose up -d
```

**默认端口**: 8899 | **默认密码**: smartping

## 设计思路

本系统的定位为轻量级工具，即使组多点成互Ping网络可以遵守无中心化原则，所有的数据均存储自身节点中，每个节点提供出方向的数据，从任意节点查询数据均会通过Ajax请求关联节点的API接口获取并组装全部数据。

## 目录结构

```
├── src/                    # Go 后端源码
│   ├── smartping.go        # 程序入口
│   ├── g/                  # 全局配置和数据结构
│   ├── http/               # HTTP 服务层
│   ├── funcs/              # 核心业务逻辑
│   ├── nettools/           # 底层网络工具
│   └── static/             # 嵌入的静态文件
│       ├── html/           # 前端页面
│       ├── conf/           # 默认配置
│       └── db/             # 默认数据库
├── web/                    # Vue 3 前端源码
│   ├── src/
│   │   ├── views/          # 页面组件
│   │   ├── components/     # 通用组件
│   │   ├── api/            # API 接口
│   │   └── assets/         # 静态资源
│   └── package.json
├── conf/                   # 配置文件（运行时生成）
├── db/                     # SQLite 数据库（运行时生成）
└── logs/                   # 日志文件（运行时生成）
```

## API 端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/config.json` | GET | 获取配置 |
| `/api/ping.json` | GET | 获取 PING 数据 |
| `/api/topology.json` | GET | 获取拓扑状态 |
| `/api/alert.json` | GET | 获取报警日志 |
| `/api/mapping.json` | GET | 获取地图数据 |
| `/api/tools.json` | GET | 在线检测工具 |
| `/api/saveconfig.json` | POST | 保存配置 |
| `/api/proxy.json` | GET | 代理访问远程节点 |

`/api/ping.json` 返回的指标数组与 `lastcheck` 时间轴逐项对应，数值使用字符串表示。没有采样记录的分钟使用 `"-"`，图表显示为断点；`"0"` 是真实记录的零值，不能与缺失采样混同。API 客户端应在转换数字前处理 `"-"`。查询时间和返回的时间轴均使用节点时区。

`/api/topology.json` 按目标 IP 返回字符串状态：`"true"` 表示未达到告警阈值，`"false"` 表示达到告警阈值，`"unknown"` 表示配置的检查窗口内没有该目标的采样（包含跨分钟完成采样的宽限时间）。未知状态不触发告警，也不将已有告警标记为恢复。客户端应显式处理三种值，不能将非 `"false"` 值一律视为正常。

## 项目贡献

欢迎参与项目贡献！比如提交PR修复一个bug，或者新建 [Issue](https://github.com/Antman2023/SmartPingNext/issues/) 讨论新特性或者变更。

## 致谢

本项目基于 [smartping/smartping](https://github.com/smartping/smartping) 开发，在原有功能基础上进行了以下改进：

- 使用 Vue 3 + TypeScript + Element Plus 重构前端
- 单文件部署，前端和默认配置嵌入二进制
- 纯 Go SQLite 驱动，无 CGO 依赖，跨平台编译
- 新增深色/浅色主题切换支持
- 可收缩侧边栏
- 中英双语界面，支持运行时切换
- 配置导入/导出功能
- Docker 镜像支持
- GitHub Actions 自动构建多平台发布包
- 图表组件防抖优化，减少频繁重绘
- 全局错误边界，提升用户体验
- 类型安全增强，移除不安全的类型断言
- 内存泄漏修复，确保组件正确销毁
- 改进响应式布局适配
- 全局 ICMP 连接池，解决并发 Ping 假丢包问题
- 修复节点删除后图表页残留显示
- 修复 ICMP DestinationUnreachable 类型断言错误
- 修复 Ping 数据查询 24 小时上限限制
- 修复最小延迟误报为 0 的计时竞态
