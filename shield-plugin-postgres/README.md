# Shield Plugin PostgreSQL

PostgreSQL Web Client plugin for [Shield CLI](https://github.com/fengyily/shield-cli) with Visual Studio Code extension support.

## 🚀 Features

### Core Features
- 📁 Browse schemas, tables, columns, and indexes
- 💻 Execute SQL queries with syntax-aware results
- 🎨 Visual table/column/index management (create, drop)
- ✏️ Insert, edit, delete rows via Web UI
- 📊 **Interactive ER Diagram** — visual schema explorer with drag-and-drop FK management
  - SVG rendering with pan, zoom, and multiple layout modes (grid / horizontal / vertical / radial)
  - Drag-and-drop foreign key creation with type matching and visual feedback
  - Click-to-select and Delete key to remove FK relations
  - Right-click context menus for table/column operations (rename, add, edit, delete)
  - Table structure editor with live SQL preview
  - Dynamic table width based on column content
  - localStorage persistence for positions, zoom, and layout
- 🔒 Read-only mode with frontend + backend enforcement
- 📤 CSV export, cell copy, column sorting
- 📦 Single binary (~6 MB), zero external dependencies

### VS Code Extension Features
- 🌐 Built-in PostgreSQL Web Client directly in VS Code
- 🔌 Connection management (add, edit, remove connections)
- 📋 Command palette integration
- 🖥️ Open in browser option for external access

## 📦 Installation

### Shield CLI Plugin

```bash
# Install plugin
shield plugin add postgres

# Connect to PostgreSQL
shield postgres 10.0.0.20:5432 --db-user postgres --db-pass mypass --database mydb
```

### VS Code Extension

1. Open VS Code
2. Go to Extensions (`Cmd+Shift+X` on macOS, `Ctrl+Shift+X` on Windows/Linux)
3. Search for "Shield CLI PostgreSQL"
4. Click "Install"
5. Reload VS Code if prompted

## 🛠️ Usage

### Shield CLI

```bash
# Connect to PostgreSQL
shield postgres <host>:<port> --db-user <username> --db-pass <password> --database <database>

# Example
shield postgres localhost:5432 --db-user postgres --db-pass mypass --database mydb
```

### VS Code Extension

1. Open VS Code
2. Press `Cmd+Shift+P` (macOS) or `Ctrl+Shift+P` (Windows/Linux) to open the command palette
3. Type "Connect to PostgreSQL" and select the command
4. Enter your connection details or select from saved connections
5. The extension will start the PostgreSQL Web Client and open it in a new tab

### Connection Management

- **Add Connection**: Use the "Add Connection" command or click the + button in the PostgreSQL Connections view
- **Edit Connection**: Right-click on a connection and select "Edit Connection"
- **Remove Connection**: Right-click on a connection and select "Remove Connection"
- **Test Connection**: Right-click on a connection and select "Test Connection"

## ⚙️ Configuration

### Shield CLI Configuration

| Parameter | Description |
|----------|-------------|
| `host` | Database host |
| `port` | Database port |
| `db-user` | Database user |
| `db-pass` | Database password |
| `database` | Default database |
| `readonly` | Read-only mode |

### VS Code Extension Configuration

The extension contributes the following settings:

* `shield-postgresql.host`: PostgreSQL host (default: localhost)
* `shield-postgresql.port`: PostgreSQL port (default: 5432)
* `shield-postgresql.user`: PostgreSQL user (default: postgres)
* `shield-postgresql.password`: PostgreSQL password
* `shield-postgresql.database`: PostgreSQL database (default: postgres)
* `shield-postgresql.readonly`: Read-only mode (default: false)

## 🏗️ Build

### Build the Plugin

```bash
go build -ldflags="-s -w" -o shield-plugin-postgres .
```

### Build the VS Code Extension

```bash
# Navigate to the VS Code extension directory
cd vscode-extension

# Install dependencies
npm install

# Build the extension
npm run package

# The extension will be built in the `dist` directory
```

## 🐳 Docker

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

## 🧪 Standalone Test

```bash
(echo '{"action":"start","config":{"host":"127.0.0.1","port":5432,"user":"postgres","pass":"mypass","database":"postgres","readonly":false}}'; cat) | ./shield-plugin-postgres
```

## 🎨 Web UI

The Web UI provides:

- **Sidebar**: Schema → Table → Column/Index tree explorer, with quick ER Diagram entry per schema
- **SQL Editor**: Multi-tab, Ctrl+Enter to execute, Tab indentation
- **Results**: Sortable columns, cell copy, CSV export, row actions
- **Structure Panel**: Column and index management with visual builders
- **ER Diagram**: Interactive entity-relationship diagram with table/column/FK CRUD operations
- **Modals**: Create schema, create table (visual column builder), create index, danger confirm with typed confirmation

## 📝 PostgreSQL-specific Adaptations

Compared to the MySQL plugin, this plugin handles:

- **Schema-based organization** instead of database-based (PG connects to one database, organizes by schemas)
- **Double-quoted identifiers** instead of backticks
- **`information_schema` queries** instead of `SHOW` commands
- **`SERIAL`/`BIGSERIAL`** instead of `AUTO_INCREMENT`
- **`SET search_path`** instead of `USE database`
- **PostgreSQL type system**: `TIMESTAMPTZ`, `JSONB`, `UUID`, `INET`, `ARRAY`, etc.
- **Parameterized queries** (`$1`, `$2`) in backend handlers

## 📖 Release Notes

### Version 0.0.1 (Initial Release)

- ✅ Core PostgreSQL Web Client functionality
- ✅ Interactive ER Diagram with drag-and-drop FK management
- ✅ VS Code Extension integration
- ✅ Connection management in VS Code
- ✅ Docker support
- ✅ Read-only mode
- ✅ CSV export and other data operations

## 📄 License

Apache 2.0

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 🌟 Support

If you encounter any issues or have questions, please open an issue on the [GitHub repository](https://github.com/fengyily/shield-plugins).

---

**Enjoy using Shield Plugin PostgreSQL!** 🎉