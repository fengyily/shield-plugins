<p align="center">
  <img src="https://raw.githubusercontent.com/fengyily/shield-cli/main/docs/logo.svg" alt="Shield CLI" width="80" height="80">
</p>

<h1 align="center">Shield Plugins</h1>

<p align="center">
  <strong>Database & service plugins for <a href="https://github.com/fengyily/shield-cli">Shield CLI</a></strong><br>
  Browser-based Web UI for PostgreSQL, Redis, SQL Server and more — managed through one CLI command.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-%3E%3D1.21-blue?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-brightgreen" alt="Platform">
  <img src="https://img.shields.io/badge/license-Apache%202.0-green" alt="License">
</p>

---

## Overview

This monorepo contains all official Shield CLI plugins. Each plugin is an **independent Go module** that compiles into a single binary (~5–9 MB), providing a browser-based Web UI for managing database or service connections.

Plugins communicate with Shield CLI via a simple **stdin/stdout JSON protocol** — no shared libraries, no complex IPC.

```
User runs: shield postgres 10.0.0.20 --db-user admin --db-pass ****

┌──────────────┐   stdin (JSON)    ┌────────────────────┐   HTTP    ┌──────────────┐
│  Shield CLI  │ ──────────────→   │  shield-plugin-xxx │ ←──────→ │  Browser UI  │
│              │ ←──────────────   │  (standalone bin)  │          │  (embedded)  │
└──────────────┘   stdout (JSON)   └────────────────────┘          └──────────────┘
       │                                    │
       │          WebSocket Tunnel          │
       └────────────────────────────────────┘
              Public HTTPS URL generated
```

## Plugins

| Plugin | Service | Protocols | Default Port | Status |
|--------|---------|-----------|:---:|--------|
| [shield-plugin-postgres](shield-plugin-postgres/) | PostgreSQL | `postgres` `pg` `postgresql` | 5432 | Development |
| shield-plugin-redis | Redis | `redis` | 6379 | Planned |
| shield-plugin-sqlserver | SQL Server | `sqlserver` `mssql` | 1433 | Planned |

> The [MySQL plugin](https://github.com/fengyily/shield-cli/tree/main/plugins/mysql) is built into the main Shield CLI repo.

## Quick Start

### Install a plugin

```bash
# From GitHub release (auto-detected platform)
shield plugin add postgres

# From local binary
shield plugin add postgres --from ./shield-plugin-postgres/shield-plugin-postgres
```

### Use it

```bash
# CLI mode — one command, browser opens
shield postgres 10.0.0.20 --db-user admin --db-pass mypass --database mydb

# Or via Web UI
shield start
# → http://localhost:8181, add PostgreSQL app, click Connect
```

### Manage plugins

```bash
shield plugin list              # List installed plugins
shield plugin upgrade postgres  # Upgrade to latest release
shield plugin remove postgres   # Uninstall
```

## Build from Source

Each plugin is an independent Go module:

```bash
cd shield-plugin-postgres
go build -ldflags="-s -w" -o shield-plugin-postgres .
```

Build all plugins:

```bash
for d in shield-plugin-*/; do
  echo "Building $d..."
  (cd "$d" && go build -ldflags="-s -w" -o "${d%/}" .)
done
```

### Cross-compile

```bash
cd shield-plugin-postgres

# Linux amd64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o shield-plugin-postgres .

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o shield-plugin-postgres.exe .

# Linux arm64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o shield-plugin-postgres .
```

## Test Locally

You can test a plugin without Shield CLI by piping JSON to stdin:

```bash
cd shield-plugin-postgres

# Interactive mode (keeps running)
(echo '{"action":"start","config":{"host":"127.0.0.1","port":5432,"user":"postgres","pass":"mypass","database":"postgres","readonly":false}}'; cat) | ./shield-plugin-postgres

# Plugin responds:
# {"status":"ready","web_port":54321,"name":"PostgreSQL Web Client","version":"0.1.0"}
#
# Open http://127.0.0.1:54321 in your browser
```

## Plugin Protocol

Plugins are standalone binaries that communicate with Shield CLI via **single-line JSON** on stdin/stdout. The protocol has only two messages:

### 1. Start — Shield CLI → Plugin (stdin)

```json
{
  "action": "start",
  "config": {
    "host": "127.0.0.1",
    "port": 5432,
    "user": "postgres",
    "pass": "secret",
    "database": "mydb",
    "readonly": false
  }
}
```

### 2. Ready / Error — Plugin → Shield CLI (stdout)

**Success:**
```json
{
  "status": "ready",
  "web_port": 54321,
  "name": "PostgreSQL Web Client",
  "version": "0.1.0"
}
```

**Error:**
```json
{
  "status": "error",
  "message": "cannot connect to PostgreSQL at 127.0.0.1:5432: connection refused"
}
```

### 3. Stop — Shield CLI → Plugin (stdin)

```json
{"action": "stop"}
```

### Protocol Rules

| Rule | Detail |
|------|--------|
| Encoding | Single-line JSON, one message per line (`json.Encoder` / `json.Decoder`) |
| Startup timeout | 15 seconds — plugin must respond within this window |
| Stop timeout | 5 seconds — graceful shutdown, then force kill |
| Web server | Plugin listens on `127.0.0.1:0` (random port), reports via `web_port` |
| Logging | stderr is forwarded to Shield CLI's log output |
| Lifecycle | Plugin runs until stdin closes, `stop` is received, or SIGINT/SIGTERM |

### Config Fields

| Field | Type | Description |
|-------|------|-------------|
| `host` | string | Target service IP/hostname |
| `port` | int | Target service port |
| `user` | string | Database/service username |
| `pass` | string | Database/service password |
| `database` | string | Database/schema name (optional) |
| `readonly` | bool | Enable read-only mode (block write SQL) |

## Creating a New Plugin

1. **Create directory**: `mkdir shield-plugin-xxx && cd shield-plugin-xxx`

2. **Init Go module**: `go mod init shield-plugin-xxx`

3. **Implement the protocol** — read JSON from stdin, start HTTP server, respond with `web_port`:

```go
package main

import (
    "database/sql"
    "embed"
    "encoding/json"
    "net"
    "net/http"
    "os"
    // import your driver
)

//go:embed static/*
var staticFS embed.FS

func main() {
    decoder := json.NewDecoder(os.Stdin)
    for {
        var req StartRequest
        if err := decoder.Decode(&req); err != nil {
            return
        }
        switch req.Action {
        case "start":
            handleStart(req.Config)
        case "stop":
            os.Exit(0)
        }
    }
}

func handleStart(cfg PluginConfig) {
    // 1. Connect to the target service
    db, err := sql.Open("yourdriver", buildDSN(cfg))
    // ...

    // 2. Find available port
    listener, _ := net.Listen("tcp", "127.0.0.1:0")
    webPort := listener.Addr().(*net.TCPAddr).Port

    // 3. Setup HTTP handlers + static files
    mux := http.NewServeMux()
    mux.HandleFunc("/api/query", queryHandler(db))
    // ...

    // 4. Respond ready
    json.NewEncoder(os.Stdout).Encode(StartResponse{
        Status:  "ready",
        WebPort: webPort,
        Name:    "Your Service Web Client",
        Version: "0.1.0",
    })

    // 5. Serve and wait
    go http.Serve(listener, mux)
    // wait for signal...
}
```

4. **Add Web UI**: Create `static/index.html` with your management interface. It will be embedded via `//go:embed static/*`.

5. **Register in Shield CLI**: Add an entry to `KnownPlugins` in [plugin/install.go](https://github.com/fengyily/shield-cli/blob/main/plugin/install.go).

## Release & Distribution

Plugins are distributed as platform-specific binaries via **GitHub Releases**.

### Asset naming convention

```
shield-plugin-{name}_{os}_{arch}.tar.gz    # Linux / macOS
shield-plugin-{name}_{os}_{arch}.zip       # Windows
```

Examples:
```
shield-plugin-postgres_linux_amd64.tar.gz
shield-plugin-postgres_darwin_arm64.tar.gz
shield-plugin-postgres_windows_amd64.zip
```

Shield CLI's `shield plugin add` command fetches the latest release from the plugin's GitHub repo, selects the correct platform asset, and extracts it to `~/.shield-cli/plugins/`.

### Plugin registry on disk

```
~/.shield-cli/plugins/
├── registry.json              # Installed plugin metadata
├── shield-plugin-mysql        # Binary
├── shield-plugin-postgres     # Binary
└── ...
```

## Repository Structure

```
shield-plugins/
├── README.md
├── .gitignore
├── shield-plugin-postgres/    ← PostgreSQL Web Client
│   ├── go.mod
│   ├── go.sum
│   ├── main.go                   Plugin protocol + HTTP server
│   ├── handler.go                API endpoints (schemas, tables, query...)
│   ├── static/
│   │   └── index.html            Embedded Web UI (~2100 lines)
│   └── README.md
├── shield-plugin-redis/       ← (planned)
└── shield-plugin-sqlserver/   ← (planned)
```

Each plugin directory is a self-contained Go module — no shared dependencies between plugins.

## License

Apache 2.0
