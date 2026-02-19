<p align="center">
    <h3 align="center">SmartPingNext | Open-source, Efficient, and Practical Network Quality Monitoring</h3>
    <p align="center">
       A comprehensive network quality (PING) monitoring tool with forward/reverse PING charts, mutual PING topology with alerts, nationwide latency map, and online diagnostic utilities.
        <br>
        <a href="./README.md">中文说明</a>
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

## Screenshot

![Screenshot](./assets/界面展示.png)

## Features

- Forward PING and reverse PING charting
- Mutual PING topology between nodes, customizable latency/loss alert thresholds (with sound), plus MTR checks on alert
- Nationwide latency map (Telecom / Unicom / Mobile lines per province)
- Built-in diagnostic tools using SmartPingNext nodes
- Node editing (modify name and IP address with automatic reference syncing)
- Configuration import/export
- Light/Dark theme switching
- Unified settings panel for theme and language
- Bilingual UI (Chinese/English) with runtime switching (no page reload)
- Collapsible sidebar
- Single-binary deployment with no extra setup files required

## Tech Stack

- **Backend**: Go 1.24 + SQLite3 (pure Go driver, no CGO dependency)
- **Frontend**: Vue 3 + TypeScript + Vite + Element Plus + ECharts

## Quick Start

### Download Release

Download the package for your platform from [Releases](https://github.com/Antman2023/SmartPingNext/releases):

| Platform | Arch | File |
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
# unzip smartping-*.zip
# double-click smartping.exe
```

On first run, the app creates `conf/`, `db/`, and `logs/` automatically and extracts default config files.

### Build from Source

```bash
# Frontend
cd web
npm install
npm run build
cp -r dist ../src/static/html

# Backend
cd ..
go build -o smartping src/smartping.go
```

### Docker

Multi-arch images are supported: `linux/amd64`, `linux/arm64`, `linux/arm/v7`

```bash
docker pull pathletboy/smartping-next:latest

# run container
docker run -d \
  --name smartping \
  -p 8899:8899 \
  -v smartping-conf:/app/conf \
  -v smartping-db:/app/db \
  -v smartping-logs:/app/logs \
  --restart unless-stopped \
  pathletboy/smartping-next:latest

# or build by yourself
docker build -t smartping-next:latest .
docker run -d --name smartping -p 8899:8899 smartping-next:latest

# or use docker-compose
docker-compose up -d
```

**Default Port**: 8899 | **Default Password**: smartping

## Design Notes

SmartPingNext is designed as a lightweight tool. Even in multi-node mutual-PING setups, it follows a decentralized approach:
- Each node stores its own data
- Each node exposes outbound monitoring data
- Querying from any node aggregates data from related nodes via AJAX API calls

## Project Structure

```text
├── src/                    # Go backend source
│   ├── smartping.go        # entry
│   ├── g/                  # global config and structs
│   ├── http/               # HTTP service layer
│   ├── funcs/              # core business logic
│   ├── nettools/           # low-level network tools
│   └── static/             # embedded static assets
│       ├── html/           # frontend bundle
│       ├── conf/           # default config
│       └── db/             # default database
├── web/                    # Vue 3 frontend source
│   ├── src/
│   │   ├── views/          # page components
│   │   ├── components/     # shared components
│   │   ├── api/            # API clients
│   │   └── assets/         # static assets
│   └── package.json
├── conf/                   # runtime config (generated at runtime)
├── db/                     # SQLite database (generated at runtime)
└── logs/                   # logs (generated at runtime)
```

## API Endpoints

| Endpoint | Method | Description |
|------|------|------|
| `/api/config.json` | GET | Get configuration |
| `/api/ping.json` | GET | Get PING data |
| `/api/topology.json` | GET | Get topology status |
| `/api/alert.json` | GET | Get alert logs |
| `/api/mapping.json` | GET | Get map latency data |
| `/api/tools.json` | GET | Online diagnostics |
| `/api/saveconfig.json` | POST | Save configuration |
| `/api/proxy.json` | GET | Proxy access to remote nodes |

## Contributing

Contributions are welcome. Feel free to open a PR for bug fixes, or create an [Issue](https://github.com/Antman2023/SmartPingNext/issues/) for feature discussions.

## Acknowledgements

This project is based on [smartping/smartping](https://github.com/smartping/smartping), with major enhancements including:

- Frontend rebuilt with Vue 3 + TypeScript + Element Plus
- Single-binary deployment with embedded frontend and default config
- Pure Go SQLite driver without CGO, enabling easier cross-platform builds
- Light/Dark theme support
- Collapsible sidebar
- Bilingual UI with runtime language switching
- Configuration import/export
- Docker image support
- GitHub Actions for multi-platform packaging
- Chart component debounce optimization for reduced redraws
- Global error boundary for improved user experience
- Enhanced type safety with removal of unsafe type assertions
- Memory leak fixes ensuring proper component cleanup
- Better responsive layout behavior
