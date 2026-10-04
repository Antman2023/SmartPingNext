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

Saving, importing and exporting wait for a successful configuration load and pause during reloads. An active save, import or export also prevents configuration reloads from overwriting the current draft. After the page unmounts, late callbacks cannot start configuration requests, password verification or saves. Edits made while a save is pending remain in the draft; success confirms only the submitted snapshot.

After a leave confirmation returns, the configuration page checks save, import and export status again so an earlier dialog cannot interrupt an operation started later. Refreshing or closing the page requests a browser leave prompt for either unsaved changes or an active operation.

Draft comparison sorts exact property names without language collation. Reordering object keys in the same configuration does not produce an unsaved warning, while array order remains significant. Editing a map entry named `__proto__` also updates the unsaved state and includes the entry in the submitted configuration.

Map names such as `__v_isReactive` and `__v_raw` remain ordinary data keys. Loading, editing, importing, exporting, saving and deleting preserve their names and carrier addresses. Additions and deletions update counts and unsaved state, while edits to other map entries keep independent pending targets valid.

Extension fields in mode, base parameters, topology settings and topology rules retain their exact keys and scalar types, including empty strings and zero values. Editing topology rules preserves existing extension fields and relative order, appending newly selected targets at the end.

Node and province deletion confirmations apply only to their original targets. Reloading, importing or replacing configuration while a confirmation is pending prevents it from deleting a new object. Switching province editors keeps the newer editor open and deletes only the originally selected province. Canceled or repeated confirmations do not report success. Deleting a node removes its incoming Ping targets and topology rules.

Node, Ping, topology and province editors retain their own targets. Closing an editor or replacing its target prevents old save callbacks from changing the draft or reporting success. Successful configuration reloads and imports close old editors; failures preserve their drafts. Ping and topology edits cannot restore removed, moved or replaced targets. Reopening an editor uses the current node list.

New node and province drafts belong to the configuration collection present when their dialogs open. Old confirmations cannot add records after closing, unmounting or configuration replacement. Validation failures retain the inputs and dialog. Adding nodes stops at 1,024; deleting a node allows retrying with the same inputs. New records apply only after saving the complete configuration.

Forward and reverse monitors retain the last successfully loaded curves for the same query when refresh fails and show the failure status. Detail views clear previous curves when the target, relative time range, or valid custom time range changes, keeping data from different queries separate.

Closed monitoring detail dialogs reject delayed queries. Leaving the page cancels active requests and refresh timers, and prevents late callbacks from reloading configuration, lists, or details. Reopening a dialog allows queries normally.

Historical Ping queries reuse ordered minute timelines to locate samples, reducing index memory for long ranges. Missing samples remain `-`, and measured zero latency and 100% loss retain their values. Non-monotonic timelines, such as those spanning a clock rollback, retain the original map-based lookup behavior.

Ping timeline labels share request-local string buffers in batches of up to 256 minutes, reducing per-minute string allocations for long queries. Previously generated labels remain unchanged, and cancellation checks, the node's timezone and clock rollback handling retain their existing behavior.

After reading the first Ping sample, timeline index preparation also checks cancellation and deadlines, with further checks every 256 labels during ordering detection and rollback-map construction. Cancellation discards unfinished indexes and closes the database cursor; successful queries still locate duplicate labels at their last occurrence.

Alert history retains each responding node's last successful records for the same date query. If a node cannot be refreshed, its records remain visible and are marked "Refresh failed". A successful response replaces those records, including an empty list. Changing the date or reloading node configuration clears previous records. When every node fails to refresh, the last successful update time is preserved.

Alert history validates archive dates and record timestamps before publishing a response. Impossible calendar dates, out-of-range time fields and invalid suffixes fail that node's refresh while preserving its previous successful data. Validation does not depend on the browser's timezone or rewrite node time labels. Stored seconds, fractional seconds, timezone suffixes and SQLite's 24-hour forms remain supported; MTR error text remains valid record content.

Alert records sort newest first by their stored calendar and time fields, including fractional seconds. Mixing spaces and `T` separators does not change the order; original labels and source failure markers remain intact. Equal times retain their original order, and 24-hour forms compare as the following day's time. Nodes do not provide timezone metadata, so sorting continues to compare their stored local time fields rather than converting records across nodes to a common absolute time.

Alert history pauses date selection and refreshes while configuration reloads, then queries the selected date using the updated node addresses and ports. Leaving the page cancels active requests and stops queued work. Late callbacks cannot start new queries or change the selected date, and an initial configuration failure can still be retried.

Map date-query and refresh callbacks also wait for configuration loading to finish. They cannot send duplicate requests to old endpoints, clear the existing map or cancel the first query started by the configuration load. Reloading preserves the selected time, and subsequent node switches use the new port. Refresh can retry an initial configuration failure.

Alert date lists seek between distinct dates using the existing date index, reducing reads when many records share a date. Dates remain newest first, and malformed timestamps retain their error handling. Sparse records incur some extra query overhead; benchmark results are recorded in the [optimization review](docs/optimization-review.md).

Hostname resolution for online tools, scheduled Ping, mapping probes, and alert MTR waits up to 5 seconds. Caller cancellation or an earlier deadline ends resolution sooner. IPv4 literals skip DNS.

Online diagnostics pause new checks while configuration reloads, then use the updated probe addresses and ports. Leaving the page cancels active requests and prevents late callbacks from starting new checks. Explicit node rejection messages in successful HTTP responses, such as rate limits or resolution failures, appear on the corresponding row. Missing rejection messages and malformed success data report an invalid response. HTTP errors and request cancellation retain their shared categories.

Successful diagnostic statistics must also report packet loss consistent with their counters. When replies exist, minimum delay must not exceed average delay, and average must not exceed maximum; a single reply must have identical delay values. Success responses with no sent probes are rejected. Valid zero delays and total-loss placeholders remain supported, and an invalid node does not prevent other nodes from finishing.

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

Before updating Ping charts, the frontend validates complete minute labels, Gregorian dates, hour/minute ranges and complete decimal metric strings. Invalid responses count as failed refreshes and preserve previous successful curves; when every source fails, the last successful update time is retained. Validation does not use the browser's timezone, reorder labels or remove duplicates caused by clock rollbacks. Valid empty responses still clear old curves.

Detailed and mini Ping charts also display the month, day, hour and minute correctly for labels with negative years. Original timelines, measurements and tooltips retain the node output without browser timezone conversion.

Mapping responses also validate `subtext` against the node's complete minute format, Gregorian dates and hour/minute ranges. Invalid responses count as failed refreshes and retain the last map and successful update time for the same query; changing the node or time clears the previous query's results. Valid empty maps clear old data, and labels are preserved without browser timezone conversion.

Explicit minute parameters for Ping and mapping queries return `406` if that local minute does not exist because of a timezone transition. The server does not silently substitute another time. Previously accepted unpadded hours and extra spaces remain supported. Alert date queries filter stored calendar labels from `00:00` on the requested date to `00:00` on the following date, avoiding shifted bounds during midnight timezone transitions.

Ping history queries use the range of all minute labels in the elapsed-time timeline, keeping samples whose labels fall outside the endpoint labels during a clock rollback. The database still stores local minute labels: repeated labels map to their last position in the timeline and cannot distinguish separate records for the two occurrences of the same local minute.

Ping history preparation honors client cancellation and deadlines. Already canceled requests skip timeline allocation; cancellation during preparation stops construction before database access and does not return partial history arrays. Parameter validation and successful response formats retain their existing rules.

Ping and alert history reuse database row scan destinations within each request, reducing temporary allocations when reading many records. Empty results do not create scan objects. Strings and alert records are still retained by value, preserving measurements, archive dates, source metadata and error handling; allocation benchmarks are recorded in the [optimization review](docs/optimization-review.md).

Archive cleanup subtracts `Base.Archive` calendar days from the node's current local date, then deletes older Ping, alert and mapping records. Records on the cutoff date and newer records are retained. Midnight clock gaps and skipped dates do not normalize the cutoff into a different calendar day.

Alert history dates and selected-day records come from the same database snapshot, avoiding a response that mixes data from before and after concurrent sampling writes or archive deletion. The snapshot ends before JSON encoding and transmission; later requests read the latest committed records.

Read-only configuration, Ping, topology, alert and mapping JSON APIs check request cancellation and deadlines before encoding and before submitting the response. Already canceled requests skip encoding; cancellation during encoding discards the result. The standard JSON encoder itself cannot be interrupted. The proxy also checks cancellation after reading the remote response, preventing the completed payload from being forwarded while releasing the body and concurrency slot.

The configuration API also checks cancellation before and during snapshot copying. Finished requests skip the full copy; cancellation during copying discards partial results and releases the read lock. Waiting for the existing configuration lock cannot be interrupted, so request status is checked again after acquiring it. Successful responses still hide the password and preserve the full configuration and existing empty collection representations.

Mapping history also checks cancellation and deadlines before decoding stored JSON, avoiding decoding and copying data for requests that have already ended. Cancellation during decoding is handled after decoding finishes, and cancellation while copying discards partial results. The standard JSON decoder itself cannot be interrupted.

Map output arrays are allocated once per populated carrier using the decoded sample count, reducing repeated growth for large results. Empty carriers still return empty arrays. Allocation starts after the first sample passes cancellation and data validation checks.

`/api/topology.json` returns string states by target IP: `"true"` means the alert threshold has not been reached, `"false"` means it has, and `"unknown"` means no samples exist for that target in the configured check window (including the grace period for rounds finishing across a minute boundary). Unknown states neither trigger alerts nor mark existing alerts as recovered. Clients must handle all three values explicitly instead of treating every non-`"false"` value as healthy.

For alert windows of at least 10 hours, the query can stop once enough bad samples among the latest records meet the occurrence threshold. The configured sample cap, minute-boundary grace and unknown state remain intact. Shorter windows retain the original query to avoid extra fixed overhead. Long windows with few or no samples incur additional overhead; measurements are recorded in the [optimization review](docs/optimization-review.md).

Node proxy responses are limited to 16 MiB, with up to 32 concurrent requests. For declared lengths between 512 bytes and 16 MiB, the proxy preallocates its read buffer to reduce growth for large responses. Actual bytes read are checked independently; truncated and oversized responses are rejected. Client cancellation stops the remote request and releases the proxy concurrency slot.

The proxy checks cancellation before allocating its read buffer. Cancelable requests read in batches of at most 32 KiB with cancellation checks, avoiding further consumption of buffered responses after cancellation. Cancellation checks and the byte limit share one reader to reduce adapter allocations. Cancellation during reading or final validation discards the result and releases the body and concurrency slot. Allocation already performed for a declared length still incurs its cost. The HTTP transport interrupts blocked network reads.

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
