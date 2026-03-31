# Shield CLI PostgreSQL VS Code Extension

Shield CLI – PostgreSQL Web Client with ER Diagram and Collaboration Features for VS Code, based on [Shield CLI](https://github.com/fengyily/shield-cli).

## 🚀 Features

- 🌐 Built-in PostgreSQL Web Client directly in VS Code
- 📁 Browse schemas, tables, columns, and indexes
- 💻 Execute SQL queries with syntax-aware results
- 🎨 Visual table/column/index management (create, drop)
- ✏️ Insert, edit, delete rows via Web UI
- 📊 **Interactive ER Diagram** — visual schema explorer with drag-and-drop FK management
- 🔒 Read-only mode with frontend + backend enforcement
- 📤 CSV export, cell copy, column sorting
- 🔌 Connection management (add, edit, remove connections)
- 📋 Command palette integration
- 🖥️ Open in browser option for external access

## 📋 Requirements

- PostgreSQL database
- Node.js (for extension development)
- Visual Studio Code version 1.107.0 or higher

## 📦 Installation

1. Open VS Code
2. Go to Extensions (`Cmd+Shift+X` on macOS, `Ctrl+Shift+X` on Windows/Linux)
3. Search for "Shield CLI PostgreSQL"
4. Click "Install"
5. Reload VS Code if prompted

## ⚙️ Extension Settings

This extension contributes the following settings:

* `shield-postgresql.host`: PostgreSQL host (default: localhost)
* `shield-postgresql.port`: PostgreSQL port (default: 5432)
* `shield-postgresql.user`: PostgreSQL user (default: postgres)
* `shield-postgresql.password`: PostgreSQL password
* `shield-postgresql.database`: PostgreSQL database (default: postgres)
* `shield-postgresql.readonly`: Read-only mode (default: false)

## 🛠️ Usage

### Basic Usage

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
- **Disconnect**: Right-click on a connected connection and select "Disconnect"
- **Open in Browser**: Right-click on a connection and select "Open in Browser" to access the Web UI in your default browser

## 🏗️ Development

### Build the Extension

```bash
# Navigate to the VS Code extension directory
cd vscode-extension

# Install dependencies
npm install

# Build the extension
npm run package

# The extension will be built in the `dist` directory
```

### Run Tests

```bash
# Run tests
npm test
```

## 📖 Release Notes

### Version 0.0.1 (Initial Release)

- ✅ Core PostgreSQL Web Client functionality
- ✅ Interactive ER Diagram with drag-and-drop FK management
- ✅ VS Code Extension integration
- ✅ Connection management in VS Code
- ✅ Read-only mode
- ✅ CSV export and other data operations

## 📄 License

Apache 2.0

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 🌟 Support

If you encounter any issues or have questions, please open an issue on the [GitHub repository](https://github.com/fengyily/shield-plugins).

---

**Enjoy using Shield CLI PostgreSQL VS Code Extension!** 🎉