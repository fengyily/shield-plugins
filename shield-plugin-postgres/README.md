# shield-plugin-postgres

PostgreSQL Web Client plugin for [Shield CLI](https://github.com/fengyily/shield-cli).

## Features

- Browse schemas, tables, columns, and indexes
- Execute SQL queries with syntax-aware results
- Visual table/column/index management (create, drop)
- Insert, edit, delete rows via Web UI
- Read-only mode with frontend + backend enforcement
- CSV export, cell copy, column sorting
- Single binary (~5 MB), zero external dependencies

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

## Standalone Test

```bash
(echo '{"action":"start","config":{"host":"127.0.0.1","port":5432,"user":"postgres","pass":"mypass","database":"postgres","readonly":false}}'; cat) | ./shield-plugin-postgres
```

## Web UI

The Web UI provides:

- **Sidebar**: Schema → Table → Column/Index tree explorer
- **SQL Editor**: Multi-tab, Ctrl+Enter to execute, Tab indentation
- **Results**: Sortable columns, cell copy, CSV export, row actions
- **Structure Panel**: Column and index management with visual builders
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
