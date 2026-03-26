# shield-plugin-postgres

PostgreSQL Web Client plugin for [Shield CLI](https://github.com/fengyily/shield-cli).

## Features

- Browse schemas, tables, columns, and indexes
- Execute SQL queries with syntax-aware results
- Visual table/column/index management (create, drop)
- Insert, edit, delete rows via Web UI
- **Interactive ER Diagram** — visual schema explorer with drag-and-drop FK management
  - SVG rendering with pan, zoom, and multiple layout modes (grid / horizontal / vertical / radial)
  - Drag-and-drop foreign key creation with type matching and visual feedback
  - Click-to-select and Delete key to remove FK relations
  - Right-click context menus for table/column operations (rename, add, edit, delete)
  - Table structure editor with live SQL preview
  - Dynamic table width based on column content
  - localStorage persistence for positions, zoom, and layout
- Read-only mode with frontend + backend enforcement
- CSV export, cell copy, column sorting
- Single binary (~6 MB), zero external dependencies

## Build

```bash
go build -ldflags="-s -w" -o shield-plugin-postgres .
```

## Usage with Shield CLI

```bash
# Install plugin
shield plugin add postgres

# Connect to PostgreSQL
shield postgres 10.0.0.20:5432 --db-user postgres --db-pass mypass --database mydb
```

## Docker

Standalone Web UI for PostgreSQL, no Shield CLI required:

```bash
docker run -d --name shield-postgres \
  -e DB_HOST=10.0.0.20 \
  -e DB_PORT=5432 \
  -e DB_USER=postgres \
  -e DB_PASS=mypass \
  -e DB_NAME=mydb \
  -p 8080:8080 \
  fengyily/shield-postgres
```

Open http://localhost:8080 — lightweight alternative to pgAdmin (~9 MB vs ~400 MB).

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `127.0.0.1` | Database host |
| `DB_PORT` | `5432` | Database port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASS` | — | Database password |
| `DB_NAME` | `postgres` | Default database |
| `DB_READONLY` | `false` | Read-only mode |
| `WEB_PORT` | `8080` | Web UI port |

## Standalone Test

```bash
(echo '{"action":"start","config":{"host":"127.0.0.1","port":5432,"user":"postgres","pass":"mypass","database":"postgres","readonly":false}}'; cat) | ./shield-plugin-postgres
```

## Web UI

The Web UI provides:

- **Sidebar**: Schema → Table → Column/Index tree explorer, with quick ER Diagram entry per schema
- **SQL Editor**: Multi-tab, Ctrl+Enter to execute, Tab indentation
- **Results**: Sortable columns, cell copy, CSV export, row actions
- **Structure Panel**: Column and index management with visual builders
- **ER Diagram**: Interactive entity-relationship diagram with table/column/FK CRUD operations
- **Modals**: Create schema, create table (visual column builder), create index, danger confirm with typed confirmation

## PostgreSQL-specific Adaptations

Compared to the MySQL plugin, this plugin handles:

- **Schema-based organization** instead of database-based (PG connects to one database, organizes by schemas)
- **Double-quoted identifiers** instead of backticks
- **`information_schema` queries** instead of `SHOW` commands
- **`SERIAL`/`BIGSERIAL`** instead of `AUTO_INCREMENT`
- **`SET search_path`** instead of `USE database`
- **PostgreSQL type system**: `TIMESTAMPTZ`, `JSONB`, `UUID`, `INET`, `ARRAY`, etc.
- **Parameterized queries** (`$1`, `$2`) in backend handlers

## License

Apache 2.0
