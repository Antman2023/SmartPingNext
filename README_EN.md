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

File logs are routed to `info.log`, `debug.log`, and `error.log`. Each file rotates before a write would exceed 10 MiB, retaining up to 3 backups (`.1` is newest, `.3` oldest); older backups are deleted. Rotation accounts for existing file sizes after a restart. A single oversized entry is kept intact and may exceed the limit. The default log level is `info`; set `SMARTPING_LOG_LEVEL` to change it. Standard output remains enabled, with retention managed by the runtime, such as Docker.

### Build from Source

Requires Go 1.24+ and Node.js 20.19+ or 22.12+.

```bash
# Frontend
cd web
npm ci
npm run build:embed

# Backend
cd ..
go build -o smartping src/smartping.go
```

`build:embed` works on Windows, Linux, and macOS. After a successful frontend build, it replaces the generated files in `src/static/html`, avoiding nested copies and stale bundled assets. Use `npm run build` when you only need the frontend output.

### Local Development and Validation

After building the frontend as above, start the built executable: `./smartping` on Linux/macOS, or build with `go build -o smartping.exe ./src` and run `.\smartping.exe` on Windows. In another terminal, enter `web` and run `npm run dev`. Open the development URL printed in the terminal (port 3000 by default); API requests are proxied to `http://localhost:8899`.

Runtime configuration, databases, and logs are stored beside the executable. Use a build at a fixed location to retain development data instead of a temporary `go run` executable.

Override these settings in `web/.env.development.local` and restart the development server:

| Variable | Default | Purpose |
|----------|---------|---------|
| `VITE_PROXY_TARGET` | `http://localhost:8899` | Backend address for the development proxy |
| `VITE_API_BASE_URL` | `/api` | Browser API prefix, including configuration and password verification |
| `VITE_API_TIMEOUT` | `15000` | Ordinary API timeout in milliseconds; remote node proxy requests allow additional response time |
| `VITE_DEFAULT_TIME_RANGE` | `6` | Default hours in forward and reverse details, from 1 minute to 31 days; invalid values fall back to 6 hours |

When using the development proxy, keep the `/api` prefix and change only `VITE_PROXY_TARGET`. The proxy preserves the browser's `Host` and `Origin` headers for the backend's configuration request origin checks. `VITE_` variables are included in frontend code and must not contain passwords. Changes for production require rebuilding both the frontend and the Go executable.

```bash
# From web: tests and lint, including local proxy integration tests
npm test
npm run lint

# From the project root: backend tests and static analysis
go test ./src/...
go vet ./src/...
```

Configuration imports accept files up to 16 MiB. Imported changes must be saved before they apply to the node. Password verification failures or timeouts during import and export preserve current edits.

Forward and reverse monitors retain the last successfully loaded curves for the same query when refresh fails and show the failure status. Detail views clear previous curves when the target, relative time range, or valid custom time range changes, keeping data from different queries separate.

Closed monitoring detail dialogs reject delayed queries. Leaving the page cancels active requests and refresh timers, and prevents late callbacks from reloading configuration, lists, or details. Reopening a dialog allows queries normally.

Historical Ping queries reuse ordered minute timelines to locate samples, reducing index memory for long ranges. Missing samples remain `-`, and measured zero latency and 100% loss retain their values. Non-monotonic timelines, such as those spanning a clock rollback, retain the original map-based lookup behavior.

Alert history retains each responding node's last successful records for the same date query. If a node cannot be refreshed, its records remain visible and are marked "Refresh failed". A successful response replaces those records, including an empty list. Changing the date or reloading node configuration clears previous records. When every node fails to refresh, the last successful update time is preserved.

Alert history pauses date selection and refreshes while configuration reloads, then queries the selected date using the updated node addresses and ports. Leaving the page cancels active requests and stops queued work. Late callbacks cannot start new queries or change the selected date, and an initial configuration failure can still be retried.

Alert date lists seek between distinct dates using the existing date index, reducing reads when many records share a date. Dates remain newest first, and malformed timestamps retain their error handling. Sparse records incur some extra query overhead; benchmark results are recorded in the [optimization review](docs/optimization-review.md).

Hostname resolution for online tools, scheduled Ping, mapping probes, and alert MTR waits up to 5 seconds. Caller cancellation or an earlier deadline ends resolution sooner. IPv4 literals skip DNS.

Online diagnostics pause new checks while configuration reloads, then use the updated probe addresses and ports. Leaving the page cancels active requests and prevents late callbacks from starting new checks. Explicit node rejection messages, such as rate limits or resolution failures, appear on the corresponding row. Missing rejection messages and malformed success data still report an invalid response.

If a refresh of the same latency map query fails, the previous result and update time remain visible with a persistent warning. A successful refresh clears the warning; switching nodes or times clears the previous query's result. Node selection pauses during configuration reloads, and late callbacks for removed nodes or an unmounted page cannot start new queries.

The topology page pauses manual refreshes while configuration reloads, then uses the updated node list, addresses and ports. Leaving the page cancels configuration and node requests, stops dispatching queued work and prevents late callbacks from starting new configuration or topology queries.

Topology refreshes update each node's status and loading markers individually, reducing repeated copies of the entire status table for large node lists. Completed nodes still update colors, links and loaded counts immediately. Failed sources show a loading error and can recover on refresh; states for sources no longer monitored are removed.

Caller cancellation and deadlines also interrupt requests waiting for ICMP initialization or shared write access. Queued senders do not block other probe responses. A probe's response timeout is still measured from its send time.

ICMP read failures are retried after 10, 20, 40, and 80 milliseconds. The fifth consecutive failure closes the connection and releases requests still waiting for a response; a later probe opens a replacement on demand. Every successful read resets the consecutive error count. An old reader cannot retire a replacement connection.

Ping, alert and mapping storage, as well as archive cleanup, also honor context cancellation and deadlines while waiting for shared database write access. Canceling queued work does not insert or delete records, and subsequent work can still acquire the lock. Queued tasks can exit promptly during service shutdown.

Service shutdown uses a 15-second grace period, stops accepting new connections, and allows existing HTTP requests to finish. Once that period expires, remaining HTTP connections are closed, canceling their requests and queued queries. An incomplete shutdown still returns an error and keeps the database open to avoid closing resources that may remain in use.

Scheduled Ping schedules the next probe at the current probe's actual start plus `PingIntervalMs`. Slow probes or scheduling delays do not cause overdue probes to run in a burst. The configured probe count is preserved, so a round may take longer.

MTR includes the initial discovery probe in each hop's cadence, so the second probe also observes the one-second interval. Subsequent probes are scheduled from each actual start, without catching up in bursts after slow probes. Hops continue sampling concurrently, and interval waits honor cancellation and deadlines. A hop that completes sampling has ten probes; the final hop at the consecutive-timeout limit retains only its discovery timeout.

On Windows, install Go and Zig on PATH, then run `pwsh -File scripts/test-race-windows.ps1` from the project root for Go race detection. The helper uses `zig cc`, adds the Windows synchronization library and a fixed loading mode for race test executables, and restores environment variables on exit. Release builds still do not require CGO. This workflow was verified with Go 1.27.1, Zig 0.16.0, and Windows amd64.

Synchronization completes a temporary copy before replacing the previous build. Copy failures leave the previous build intact, and replacement failures trigger a rollback. If rollback also fails, the command reports the retained backup path for manual recovery.

For deployment under a path such as `/smartping/`, set `VITE_API_BASE_URL=/smartping/api` in `web/.env.local`, then run from `web/`:

```bash
npm run build -- --base=/smartping/
node scripts/sync-embed.mjs
```

Rebuild the backend afterward. Configure your reverse proxy to forward requests under `/smartping/` to SmartPing with that prefix removed (for example, `/smartping/api/config.json` becomes `/api/config.json`). Frontend routes and static assets use the base path specified at build time.

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

The metric arrays returned by `/api/ping.json` align with the `lastcheck` timeline and contain string values. Minutes without a stored sample use `"-"` and appear as gaps in charts. `"0"` is a recorded zero value, distinct from missing data. API clients should handle `"-"` before converting values to numbers. Query times and the returned timeline use the node's timezone.

Explicit minute parameters for Ping and mapping queries return `406` if that local minute does not exist because of a timezone transition. The server does not silently substitute another time. Previously accepted unpadded hours and extra spaces remain supported. Alert date queries filter stored calendar labels from `00:00` on the requested date to `00:00` on the following date, avoiding shifted bounds during midnight timezone transitions.

`/api/topology.json` returns string states by target IP: `"true"` means the alert threshold has not been reached, `"false"` means it has, and `"unknown"` means no samples exist for that target in the configured check window (including the grace period for rounds finishing across a minute boundary). Unknown states neither trigger alerts nor mark existing alerts as recovered. Clients must handle all three values explicitly instead of treating every non-`"false"` value as healthy.

Node proxy responses are limited to 16 MiB, with up to 32 concurrent requests. For declared lengths between 512 bytes and 16 MiB, the proxy preallocates its read buffer to reduce growth for large responses. Actual bytes read are checked independently; truncated and oversized responses are rejected. Client cancellation stops the remote request and releases the proxy concurrency slot.

Node JSON APIs and proxy responses declare `Content-Length` using the encoded byte count, allowing large responses to use the preallocated read path. Password and online tool rate-limit responses also declare accurate lengths while preserving status `429`, `Retry-After` and their existing JSON fields.

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
- Global ICMP connection pool to eliminate false packet loss under concurrent pings
- Fix deleted nodes still appearing on chart pages
- Fix ICMP DestinationUnreachable type assertion error
- Fix 24-hour upper limit on ping data queries
- Fix minimum delay falsely reported as 0 due to timing race
