# Shield Plugins — Claude Code Context

## Project Overview

Monorepo for [Shield CLI](https://github.com/fengyily/shield-cli) database/service plugins. Each plugin is a **standalone Go binary** (~9MB) providing a browser-based Web UI, communicating with Shield CLI via stdin/stdout JSON protocol.

## Repository Structure

```
shield-plugins/
├── shield-plugin-postgres/     ← Active, primary plugin
│   ├── main.go                 Plugin protocol + HTTP server + go:embed
│   ├── handler.go              REST API endpoints
│   ├── collab.go               WebSocket hub for ER collaboration
│   ├── static/
│   │   ├── index.html          Full Web UI (HTML + CSS + inline JS)
│   │   └── er.js               ER diagram module (~2800 lines)
│   ├── vscode-extension/       VS Code extension (TypeScript)
│   ├── Dockerfile              Multi-stage → scratch (~9MB)
│   └── docker-compose.yml
├── shield-plugin-redis/        Planned (empty)
├── shield-plugin-filetransfer/ Planned (empty)
└── .github/workflows/
    ├── ci.yml                  Build on push/PR
    └── release.yml             Multi-platform release + Docker
```

## Tech Stack

- **Backend**: Go 1.25, `lib/pq`, `gorilla/websocket`
- **Frontend**: Vanilla JS (no frameworks), pure SVG for ER diagrams
- **Static embedding**: `//go:embed static/*` — zero external file dependencies
- **VS Code Extension**: TypeScript, esbuild

## Key Architecture Decisions

- **No frontend framework** — single `index.html` + `er.js`, all embedded in Go binary
- **Plugin protocol**: single-line JSON on stdin/stdout, random port HTTP server
- **ER diagram**: pure SVG string generation (no D3/Canvas), RAF-batched rendering
- **Collaboration**: WebSocket with cursor/drag/viewport sync, follow mode
- **Read-only mode**: dual enforcement (frontend hides UI + backend rejects write SQL)

## Building

```bash
cd shield-plugin-postgres
go build -ldflags="-s -w" -o shield-plugin-postgres .
```

## Testing Locally

```bash
cd shield-plugin-postgres
(echo '{"action":"start","config":{"host":"127.0.0.1","port":5432,"user":"postgres","pass":"xxx","database":"mydb","readonly":false}}'; cat) | ./shield-plugin-postgres
# Opens web UI at the port in the JSON response
```

## ER Diagram (er.js) Architecture

The ER diagram is the most complex frontend component. Key subsystems:

### State
- `erData` — schema from `/api/er` (tables, columns, relations)
- `tablePositions` — `{ name: {x, y} }`, persisted to localStorage
- `displayMode` — `'compact'` (default) or `'name-only'`, per-table expandable
- `focusTable` / `focusSet` — focus mode (N-degree BFS subset)
- `hoveredTable` — for relation line highlighting
- `currentLayout` — `'relations'` (default), `'grid'`, `'horizontal'`, `'vertical'`, `'center'`

### Layout Algorithms
- **Grid**: width-aware row wrapping (`placeGrid`)
- **Horizontal/Vertical**: topological layers with canvas-aware overflow (`placeLayers`)
- **Center**: radial BFS rings with adaptive radius
- **Relations (FK)**: hub + BFS layers, FK-column-aligned Y, left/right alternating, multi-column overflow (`relPlaceMultiCol`)

### Rendering Pipeline
`loadER()` → `computeAllWidths()` → `runLayout()` → `fitToView()` → `renderSvg()`

Each render builds complete SVG string → `canvas.innerHTML = svg` (batched via `scheduleRender` / RAF).

### Key Functions
| Function | Purpose |
|----------|---------|
| `renderSvg()` | Master render orchestrator |
| `renderTableSvg(t)` | Single table SVG (header, columns, icons, expand/collapse) |
| `renderRelations()` | FK lines with hover highlight and focus filtering |
| `setFocus(name, depth)` | Enter focus mode with relayout + viewport animation |
| `focusRelayout()` | Re-layout focus subset using FK algorithm |
| `relPlaceMultiCol()` | Place tables in columns with height overflow |
| `erSearchTable()` | Fuzzy search with dropdown results |
| `erGoToTable()` | Viewport animation to target table |
| `animateViewportTo()` | 120ms ease-out viewport transition |

### Interaction Model
- **Left click**: select table, start FK drag (on column), icon actions
- **Right click / drag**: pan canvas, context menu
- **Double click**: toggle focus mode
- **Scroll**: pan (plain) / zoom (Ctrl+scroll)
- **Cmd+F**: search tables
- **Escape**: exit focus → close ER overlay

### Collaboration (WebSocket)
Messages: `welcome`, `presence`, `cursor`, `drag`, `viewport`, `schema_changed`

## Web UI (index.html) Structure

Single file containing:
- CSS variables (`:root` theme)
- All component styles
- HTML: header, sidebar tree, SQL editor, result area, ER overlay
- Inline JS: tree rendering, tab management, query execution, schema operations

## API Endpoints (handler.go)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/info` | GET | Server version, readonly status |
| `/api/schemas` | GET | List schemas |
| `/api/tables` | GET | Tables in schema |
| `/api/columns` | GET | Column details |
| `/api/indexes` | GET | Index info |
| `/api/query` | POST | Execute SQL (write blocked in readonly) |
| `/api/er` | GET | ER diagram data (tables + relations) |
| `/api/export` | GET | SQL export (schema/table DDL) |
| `/ws/er` | WS | Real-time collaboration |

## Release Process

- Tag `vX.Y.Z` → GitHub Actions builds multi-platform binaries + Docker images
- Docker: `fengyily/shield-postgres` on Docker Hub + GHCR
- Platforms: linux/darwin/windows × amd64/arm64

## Conventions

- Commit style: `feat(postgres):`, `fix(postgres):`, `ui(postgres):`
- No frontend build step — edit `index.html` / `er.js` directly
- All frontend state is in the IIFE closure in `er.js`
- CSS uses `var(--xxx)` theme variables defined in `:root`
