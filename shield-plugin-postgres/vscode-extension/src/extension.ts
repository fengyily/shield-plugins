// The module 'vscode' contains the VS Code extensibility API
// Import the module and reference it with the alias vscode in your code below
import * as vscode from 'vscode';
import * as child_process from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

let postgresProcess: child_process.ChildProcess | null = null;
let webPort: number | null = null;

// Connection interface
interface Connection {
	id: string;
	name: string;
	host: string;
	port: number;
	user: string;
	password: string;
	database: string;
	readonly: boolean;
}

// Connection storage
class ConnectionStorage {
	private context: vscode.ExtensionContext;
	private key = 'shield-postgresql.connections';

	constructor(context: vscode.ExtensionContext) {
		this.context = context;
	}

	getConnections(): Connection[] {
		return this.context.globalState.get<Connection[]>(this.key, []);
	}

	saveConnections(connections: Connection[]): void {
		this.context.globalState.update(this.key, connections);
	}

	addConnection(connection: Connection): void {
		const connections = this.getConnections();
		connections.push(connection);
		this.saveConnections(connections);
	}

	removeConnection(id: string): void {
		const connections = this.getConnections();
		const filtered = connections.filter(c => c.id !== id);
		this.saveConnections(filtered);
	}

	updateConnection(updatedConnection: Connection): void {
		const connections = this.getConnections();
		const index = connections.findIndex(c => c.id === updatedConnection.id);
		if (index !== -1) {
			connections[index] = updatedConnection;
			this.saveConnections(connections);
		}
	}
}

// Tree data provider for connections
class ConnectionTreeDataProvider implements vscode.TreeDataProvider<ConnectionItem> {
	private _onDidChangeTreeData: vscode.EventEmitter<ConnectionItem | undefined | null | void> = new vscode.EventEmitter<ConnectionItem | undefined | null | void>();
	onDidChangeTreeData: vscode.Event<ConnectionItem | undefined | null | void> = this._onDidChangeTreeData.event;

	private storage: ConnectionStorage;

	constructor(storage: ConnectionStorage) {
		this.storage = storage;
	}

	reload(): void {
		this._onDidChangeTreeData.fire();
	}

	getTreeItem(element: ConnectionItem): vscode.TreeItem {
		return element;
	}

	getChildren(element?: ConnectionItem): Thenable<ConnectionItem[]> {
		if (element) {
			return Promise.resolve([]);
		} else {
			const connections = this.storage.getConnections();
			return Promise.resolve(connections.map(c => new ConnectionItem(c)));
		}
	}
}

// Connection tree item
class ConnectionItem extends vscode.TreeItem {
	constructor(public connection: Connection) {
		super(connection.name, vscode.TreeItemCollapsibleState.None);
		this.id = connection.id;
		
		// Get connection status
		const status = connectionStatus[connection.id] || 'disconnected';
		
		// Set description with status
		this.description = `${connection.host}:${connection.port}/${connection.database} (${status})`;
		// Set contextValue based on status
		this.contextValue = status === 'connected' ? 'connection.connected' : 'connection';
		
		// Set icon based on status
		switch (status) {
			case 'connected':
				this.iconPath = new vscode.ThemeIcon('database');
				break;
			case 'connecting':
				this.iconPath = new vscode.ThemeIcon('sync~spin');
				break;
			default:
				this.iconPath = new vscode.ThemeIcon('database');
				break;
		}
		
		// Add double click command
		if (status === 'connected') {
			this.command = {
				command: 'shield-postgresql.connectFromTree',
				title: 'Open Connection',
				arguments: [this]
			};
			// Set hover message
			this.tooltip = 'Disconnect from PostgreSQL';
		} else {
			this.command = {
				command: 'shield-postgresql.connectFromTree',
				title: 'Connect',
				arguments: [this]
			};
			// Set hover message
			this.tooltip = 'Connect to PostgreSQL';
		};
	}
}

// Connection status tracking
interface ConnectionStatus {
	[id: string]: 'connected' | 'disconnected' | 'connecting';
}

// Connection info tracking
interface ConnectionInfo {
	port: number;
	process: child_process.ChildProcess;
}

let connectionStatus: ConnectionStatus = {};
let connectionInfo: Record<string, ConnectionInfo> = {};

// This method is called when your extension is activated
// Your extension is activated the very first time the command is executed
export function activate(context: vscode.ExtensionContext) {

	// Use the console to output diagnostic information (console.log) and errors (console.error)
	// This line of code will only be executed once when your extension is activated
	console.log('Congratulations, your extension "shield-postgresql" is now active!');

	// Initialize connection storage
	const storage = new ConnectionStorage(context);

	// Create tree data provider
	const treeDataProvider = new ConnectionTreeDataProvider(storage);

	// Register tree view
	vscode.window.registerTreeDataProvider('shield-postgresql.views.postgresql', treeDataProvider);

	// Register commands
	const connectDisposable = vscode.commands.registerCommand('shield-postgresql.connect', async () => {
		// Get configuration
		const config = vscode.workspace.getConfiguration('shield-postgresql');
		const host = config.get<string>('host', 'localhost');
		const port = config.get<number>('port', 5432);
		const user = config.get<string>('user', 'postgres');
		const password = config.get<string>('password', '');
		const database = config.get<string>('database', 'postgres');
		const readonly = config.get<boolean>('readonly', false);

		// Get platform-specific executable name
		const platform = process.platform;
		let executableName = 'shield-plugin-postgres';
		if (platform === 'win32') {
			executableName += '.exe';
		} else if (platform === 'linux') {
			executableName += '-linux';
		}

		// Check if shield-plugin-postgres executable exists
		const pluginPath = path.join(__dirname, '..', 'bin', executableName);
		if (!fs.existsSync(pluginPath)) {
			vscode.window.showErrorMessage('shield-plugin-postgres executable not found for your platform. Please contact the plugin author.');
			return;
		}

		// Stop existing process if running
		if (postgresProcess) {
			postgresProcess.kill();
			postgresProcess = null;
			webPort = null;
		}

		// Start shield-plugin-postgres process
		try {
			// Create start request
			const startRequest = {
				action: 'start',
				config: {
					host,
					port,
					user,
					pass: password,
					database,
					readonly
				}
			};

			// Start the process
			postgresProcess = child_process.spawn(pluginPath, [], {
				stdio: ['pipe', 'pipe', 'pipe']
			});

			// Write start request to stdin
			postgresProcess.stdin?.write(JSON.stringify(startRequest) + '\n');
			postgresProcess.stdin?.end();

			// Read response from stdout
			let response = '';
			let responseReceived = false;

			// Show connecting status
			const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
			statusBarItem.text = '$(sync~spin) Connecting to PostgreSQL...';
			statusBarItem.show();

			// Wait for response
			await new Promise<void>((resolve, reject) => {
				// Handle stdout data
				postgresProcess?.stdout?.on('data', (data) => {
					response += data.toString();
					// Try to parse response as soon as we get data
					try {
						const result = JSON.parse(response);
						if (result.status === 'ready') {
							webPort = result.web_port;
							// Open WebView
							openPostgreSQLWebView(context, webPort!);
							responseReceived = true;
							statusBarItem.text = '$(database) PostgreSQL Connected';
							statusBarItem.show();
							setTimeout(() => statusBarItem.hide(), 3000);
							resolve();
						} else {
							vscode.window.showErrorMessage(`Failed to connect: ${result.message}`);
							responseReceived = true;
							statusBarItem.text = '$(x) PostgreSQL Connection Failed';
							statusBarItem.show();
							setTimeout(() => statusBarItem.hide(), 3000);
							reject(new Error(result.message));
						}
					} catch (e) {
						// Ignore parse errors until we have complete response
					}
				});

				// Handle process errors
				postgresProcess?.stderr?.on('data', (data) => {
					console.error('shield-plugin-postgres error:', data.toString());
				});

				// Handle process exit
				postgresProcess?.on('exit', (code) => {
					console.log('shield-plugin-postgres exited with code:', code);
					if (!responseReceived) {
						reject(new Error(`shield-plugin-postgres exited with code ${code} without sending a response`));
						statusBarItem.text = '$(x) PostgreSQL Connection Failed';
						statusBarItem.show();
						setTimeout(() => statusBarItem.hide(), 3000);
					}
					postgresProcess = null;
					webPort = null;
				});

				// Timeout after 10 seconds
				setTimeout(() => {
					reject(new Error('Timeout waiting for shield-plugin-postgres response'));
					statusBarItem.text = '$(x) PostgreSQL Connection Timeout';
					statusBarItem.show();
					setTimeout(() => statusBarItem.hide(), 3000);
				}, 10000);
			});
		} catch (error) {
			console.error('Error starting shield-plugin-postgres:', error);
			vscode.window.showErrorMessage(`Error starting PostgreSQL client: ${error instanceof Error ? error.message : String(error)}`);
		}
	});

	const addConnectionDisposable = vscode.commands.registerCommand('shield-postgresql.addConnection', async () => {
		// Create webview panel
		const panel = vscode.window.createWebviewPanel(
			'shield-postgresql.addConnection',
			'Add PostgreSQL Connection',
			vscode.ViewColumn.One,
			{
				enableScripts: true,
				localResourceRoots: [vscode.Uri.joinPath(context.extensionUri, 'src')]
			}
		);

		// Webview HTML content
		panel.webview.html = `
			<!DOCTYPE html>
			<html lang="en">
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>Add PostgreSQL Connection</title>
				<style>
					body {
						font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
						padding: 20px;
						background-color: #f5f5f5;
					}
					.container {
						max-width: 500px;
						margin: 0 auto;
						background-color: white;
						padding: 24px;
						border-radius: 8px;
						box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
					}
					h1 {
								font-size: 20px;
								margin-bottom: 20px;
								color: #333;
							}
							.intro {
								background-color: #e7f3ff;
								border-left: 4px solid #336791;
								padding: 12px;
								margin-bottom: 20px;
								border-radius: 4px;
							}
							.intro p {
								margin: 0 0 8px 0;
								color: #0c5460;
								font-size: 14px;
							}
							.intro p:last-child {
								margin-bottom: 0;
							}
							.form-group {
						margin-bottom: 16px;
					}
					label {
						display: block;
						margin-bottom: 6px;
						font-weight: 500;
						color: #555;
					}
					input[type="text"],
					input[type="password"],
					input[type="number"] {
						width: 100%;
						padding: 8px 12px;
						border: 1px solid #ddd;
						border-radius: 4px;
						font-size: 14px;
						box-sizing: border-box;
					}
					input[type="text"]:focus,
					input[type="password"]:focus,
					input[type="number"]:focus {
						outline: none;
						border-color: #336791;
						box-shadow: 0 0 0 2px rgba(51, 103, 145, 0.2);
					}
					.checkbox-group {
						display: flex;
						align-items: center;
					}
					input[type="checkbox"] {
						margin-right: 8px;
					}
					.button-group {
						margin-top: 24px;
						display: flex;
						gap: 12px;
					}
					button {
						padding: 8px 16px;
						border: none;
						border-radius: 4px;
						font-size: 14px;
						font-weight: 500;
						cursor: pointer;
						transition: background-color 0.2s;
					}
					button.primary {
						background-color: #336791;
						color: white;
					}
					button.primary:hover {
						background-color: #2a557a;
					}
					button.secondary {
						background-color: #f0f0f0;
						color: #333;
					}
					button.secondary:hover {
						background-color: #e0e0e0;
					}
					.error {
						color: #e74c3c;
						font-size: 12px;
						margin-top: 4px;
					}
				</style>
			</head>
			<body>
				<div class="container">
						<h1>Add PostgreSQL Connection</h1>
						<div class="intro">
							<p>Enter the connection details for your PostgreSQL database. This will allow you to connect to your database and manage it using Shield PostgreSQL.</p>
							<p><strong>Note:</strong> The password will be stored securely in your VS Code settings.</p>
						</div>
						<form id="connectionForm">
							<div class="form-group">
							<label for="name">Connection Name *</label>
							<input type="text" id="name" name="name" placeholder="Enter a name for this connection" value="">
							<div class="error" id="nameError"></div>
						</div>
						<div class="form-group">
							<label for="host">Host *</label>
							<input type="text" id="host" name="host" placeholder="localhost" value="localhost">
							<div class="error" id="hostError"></div>
						</div>
						<div class="form-group">
							<label for="port">Port *</label>
							<input type="number" id="port" name="port" placeholder="5432" value="5432" min="1" max="65535">
							<div class="error" id="portError"></div>
						</div>
						<div class="form-group">
							<label for="user">User *</label>
							<input type="text" id="user" name="user" placeholder="postgres" value="postgres">
							<div class="error" id="userError"></div>
						</div>
						<div class="form-group">
							<label for="password">Password</label>
							<input type="password" id="password" name="password" placeholder="Enter password" value="">
						</div>
						<div class="form-group">
							<label for="database">Database *</label>
							<input type="text" id="database" name="database" placeholder="postgres" value="postgres">
							<div class="error" id="databaseError"></div>
						</div>
						<div class="form-group checkbox-group">
							<input type="checkbox" id="readonly" name="readonly" value="true">
							<label for="readonly">Read-only mode</label>
						</div>
						<div class="button-group">
							<button type="button" class="secondary" id="cancelButton">Cancel</button>
							<button type="submit" class="primary">Save</button>
						</div>
					</form>
				</div>
				<script>
					const vscode = acquireVsCodeApi();

					document.getElementById('cancelButton').addEventListener('click', () => {
						vscode.postMessage({ type: 'cancel' });
					});

					document.getElementById('connectionForm').addEventListener('submit', (e) => {
						e.preventDefault();
						
						// Validate form
						let isValid = true;
						
						// Clear previous errors
						const errorElements = document.querySelectorAll('.error');
						errorElements.forEach(el => el.textContent = '');
						
						// Validate required fields
						const name = document.getElementById('name').value.trim();
						if (!name) {
							document.getElementById('nameError').textContent = 'Connection name is required';
							isValid = false;
						}
						
						const host = document.getElementById('host').value.trim();
						if (!host) {
							document.getElementById('hostError').textContent = 'Host is required';
							isValid = false;
						}
						
						const port = document.getElementById('port').value;
						if (!port || isNaN(port) || port < 1 || port > 65535) {
							document.getElementById('portError').textContent = 'Valid port is required (1-65535)';
							isValid = false;
						}
						
						const user = document.getElementById('user').value.trim();
						if (!user) {
							document.getElementById('userError').textContent = 'User is required';
							isValid = false;
						}
						
						const database = document.getElementById('database').value.trim();
						if (!database) {
							document.getElementById('databaseError').textContent = 'Database is required';
							isValid = false;
						}
						
						if (isValid) {
							const formData = {
								name: name,
								host: host,
								port: port,
								user: user,
								password: document.getElementById('password').value,
								database: database,
								readonly: document.getElementById('readonly').checked
							};
							
							vscode.postMessage({ type: 'save', data: formData });
						}
					});
				</script>
			</body>
			</html>
		`;

		// Handle messages from webview
		panel.webview.onDidReceiveMessage(async (message) => {
			switch (message.type) {
				case 'save': {
					const formData = message.data;
					
					// Create new connection
					const newConnection: Connection = {
						id: Date.now().toString(),
						name: formData.name,
						host: formData.host,
						port: parseInt(formData.port),
						user: formData.user,
						password: formData.password,
						database: formData.database,
						readonly: formData.readonly
					};

					// Save connection
					storage.addConnection(newConnection);
					treeDataProvider.reload();

					vscode.window.showInformationMessage(`Connection "${formData.name}" added successfully`);
					panel.dispose();
					break;
				}
				case 'cancel': {
					panel.dispose();
					break;
				}
			}
		});
	});

	const removeConnectionDisposable = vscode.commands.registerCommand('shield-postgresql.removeConnection', async (item: ConnectionItem) => {
		if (item) {
			const confirmed = await vscode.window.showQuickPick(['Yes', 'No'], { prompt: `Are you sure you want to remove connection "${item.connection.name}"?` });
			if (confirmed === 'Yes') {
				storage.removeConnection(item.connection.id);
				treeDataProvider.reload();
				vscode.window.showInformationMessage(`Connection "${item.connection.name}" removed successfully`);
			}
		}
	});

	const editConnectionDisposable = vscode.commands.registerCommand('shield-postgresql.editConnection', async (item: ConnectionItem) => {
		if (item) {
			const connection = item.connection;

			// Create webview panel
			const panel = vscode.window.createWebviewPanel(
				'shield-postgresql.editConnection',
				'Edit PostgreSQL Connection',
				vscode.ViewColumn.One,
				{
					enableScripts: true,
					localResourceRoots: [vscode.Uri.joinPath(context.extensionUri, 'src')]
				}
			);

			// Webview HTML content
			panel.webview.html = `
				<!DOCTYPE html>
				<html lang="en">
				<head>
					<meta charset="UTF-8">
					<meta name="viewport" content="width=device-width, initial-scale=1.0">
					<title>Edit PostgreSQL Connection</title>
					<style>
						body {
							font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
							padding: 20px;
							background-color: #f5f5f5;
						}
						.container {
							max-width: 500px;
							margin: 0 auto;
							background-color: white;
							padding: 24px;
							border-radius: 8px;
							box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
						}
						h1 {
								font-size: 20px;
								margin-bottom: 20px;
								color: #333;
							}
							.intro {
								background-color: #e7f3ff;
								border-left: 4px solid #336791;
								padding: 12px;
								margin-bottom: 20px;
								border-radius: 4px;
							}
							.intro p {
								margin: 0 0 8px 0;
								color: #0c5460;
								font-size: 14px;
							}
							.intro p:last-child {
								margin-bottom: 0;
							}
							.form-group {
							margin-bottom: 16px;
						}
						label {
							display: block;
							margin-bottom: 6px;
							font-weight: 500;
							color: #555;
						}
						input[type="text"],
						input[type="password"],
						input[type="number"] {
							width: 100%;
							padding: 8px 12px;
							border: 1px solid #ddd;
							border-radius: 4px;
							font-size: 14px;
							box-sizing: border-box;
						}
						input[type="text"]:focus,
						input[type="password"]:focus,
						input[type="number"]:focus {
							outline: none;
							border-color: #336791;
							box-shadow: 0 0 0 2px rgba(51, 103, 145, 0.2);
						}
						.checkbox-group {
							display: flex;
							align-items: center;
						}
						input[type="checkbox"] {
							margin-right: 8px;
						}
						.button-group {
							margin-top: 24px;
							display: flex;
							gap: 12px;
						}
						button {
							padding: 8px 16px;
							border: none;
							border-radius: 4px;
							font-size: 14px;
							font-weight: 500;
							cursor: pointer;
							transition: background-color 0.2s;
						}
						button.primary {
							background-color: #336791;
							color: white;
						}
						button.primary:hover {
							background-color: #2a557a;
						}
						button.secondary {
							background-color: #f0f0f0;
							color: #333;
						}
						button.secondary:hover {
							background-color: #e0e0e0;
						}
						.error {
							color: #e74c3c;
							font-size: 12px;
							margin-top: 4px;
						}
					</style>
				</head>
				<body>
					<div class="container">
						<h1>Edit PostgreSQL Connection</h1>
						<div class="intro">
							<p>Update the connection details for your PostgreSQL database. Changes will be saved automatically.</p>
							<p><strong>Note:</strong> The password will be stored securely in your VS Code settings.</p>
						</div>
						<form id="connectionForm">
							<div class="form-group">
								<label for="name">Connection Name *</label>
								<input type="text" id="name" name="name" placeholder="Enter a name for this connection" value="${connection.name}">
								<div class="error" id="nameError"></div>
							</div>
							<div class="form-group">
								<label for="host">Host *</label>
								<input type="text" id="host" name="host" placeholder="localhost" value="${connection.host}">
								<div class="error" id="hostError"></div>
							</div>
							<div class="form-group">
								<label for="port">Port *</label>
								<input type="number" id="port" name="port" placeholder="5432" value="${connection.port}" min="1" max="65535">
								<div class="error" id="portError"></div>
							</div>
							<div class="form-group">
								<label for="user">User *</label>
								<input type="text" id="user" name="user" placeholder="postgres" value="${connection.user}">
								<div class="error" id="userError"></div>
							</div>
							<div class="form-group">
								<label for="password">Password</label>
								<input type="password" id="password" name="password" placeholder="Enter password" value="${connection.password}">
							</div>
							<div class="form-group">
								<label for="database">Database *</label>
								<input type="text" id="database" name="database" placeholder="postgres" value="${connection.database}">
								<div class="error" id="databaseError"></div>
							</div>
							<div class="form-group checkbox-group">
								<input type="checkbox" id="readonly" name="readonly" value="true" ${connection.readonly ? 'checked' : ''}>
								<label for="readonly">Read-only mode</label>
							</div>
							<div class="button-group">
								<button type="button" class="secondary" id="cancelButton">Cancel</button>
								<button type="submit" class="primary">Save</button>
							</div>
						</form>
					</div>
					<script>
						const vscode = acquireVsCodeApi();

						document.getElementById('cancelButton').addEventListener('click', () => {
							vscode.postMessage({ type: 'cancel' });
						});

						document.getElementById('connectionForm').addEventListener('submit', (e) => {
							e.preventDefault();
							
							// Validate form
							let isValid = true;
							
							// Clear previous errors
							const errorElements = document.querySelectorAll('.error');
							errorElements.forEach(el => el.textContent = '');
							
							// Validate required fields
							const name = document.getElementById('name').value.trim();
							if (!name) {
								document.getElementById('nameError').textContent = 'Connection name is required';
								isValid = false;
							}
							
							const host = document.getElementById('host').value.trim();
							if (!host) {
								document.getElementById('hostError').textContent = 'Host is required';
								isValid = false;
							}
							
							const port = document.getElementById('port').value;
							if (!port || isNaN(port) || port < 1 || port > 65535) {
								document.getElementById('portError').textContent = 'Valid port is required (1-65535)';
								isValid = false;
							}
							
							const user = document.getElementById('user').value.trim();
							if (!user) {
								document.getElementById('userError').textContent = 'User is required';
								isValid = false;
							}
							
							const database = document.getElementById('database').value.trim();
							if (!database) {
								document.getElementById('databaseError').textContent = 'Database is required';
								isValid = false;
							}
							
							if (isValid) {
								const formData = {
									name: name,
									host: host,
									port: port,
									user: user,
									password: document.getElementById('password').value,
									database: database,
									readonly: document.getElementById('readonly').checked
								};
								
								vscode.postMessage({ type: 'save', data: formData });
							}
						});
					</script>
				</body>
				</html>
			`;

			// Handle messages from webview
			panel.webview.onDidReceiveMessage(async (message) => {
				switch (message.type) {
					case 'save': {
						const formData = message.data;
						
						// Update connection
						const updatedConnection: Connection = {
							...connection,
							name: formData.name,
							host: formData.host,
							port: parseInt(formData.port),
							user: formData.user,
							password: formData.password,
							database: formData.database,
							readonly: formData.readonly
						};

						// Save updated connection
						storage.updateConnection(updatedConnection);
						treeDataProvider.reload();

						vscode.window.showInformationMessage(`Connection "${formData.name}" updated successfully`);
						panel.dispose();
						break;
					}
					case 'cancel': {
						panel.dispose();
						break;
					}
				}
			});
		}
	});

	// Add command to connect from tree item
	const connectFromTreeDisposable = vscode.commands.registerCommand('shield-postgresql.connectFromTree', async (item: ConnectionItem) => {
		if (item) {
			const connection = item.connection;

			// Check if connection is already established
			if (connectionStatus[connection.id] === 'connected' && connectionInfo[connection.id]) {
				// Open existing URL in VS Code webview
				const url = `http://localhost:${connectionInfo[connection.id].port}`;
				const panel = vscode.window.createWebviewPanel(
					'shield-postgresql.connection',
					`PostgreSQL - ${connection.name}`,
					vscode.ViewColumn.One,
					{
						enableScripts: true,
						enableCommandUris: true,
						localResourceRoots: [vscode.Uri.joinPath(context.extensionUri, 'src')]
					}
				);
				panel.webview.html = `
					<!DOCTYPE html>
					<html lang="en">
					<head>
						<meta charset="UTF-8">
						<meta name="viewport" content="width=device-width, initial-scale=1.0">
						<title>PostgreSQL Connection</title>
						<style>
							body {
								margin: 0;
								padding: 0;
								overflow: hidden;
							}
							iframe {
								width: 100%;
								height: 100vh;
								border: none;
							}
						</style>
					</head>
					<body>
						<iframe src="${url}"></iframe>
					</body>
					</html>
				`;
				return;
			}

			// Update connection status to connecting
			connectionStatus[connection.id] = 'connecting';
			treeDataProvider.reload();

			// Get platform-specific executable name
			const platform = process.platform;
			let executableName = 'shield-plugin-postgres';
			if (platform === 'win32') {
				executableName += '.exe';
			} else if (platform === 'linux') {
				executableName += '-linux';
			}

			// Check if shield-plugin-postgres executable exists
			const pluginPath = path.join(__dirname, '..', 'bin', executableName);
			if (!fs.existsSync(pluginPath)) {
				vscode.window.showErrorMessage('shield-plugin-postgres executable not found for your platform. Please contact the plugin author.');
				// Update connection status to disconnected
				connectionStatus[connection.id] = 'disconnected';
				treeDataProvider.reload();
				return;
			}

			// Stop existing process for this connection if running
			if (connectionInfo[connection.id]) {
				connectionInfo[connection.id].process.kill();
				delete connectionInfo[connection.id];
			}

			// Start shield-plugin-postgres process
			try {
				// Create start request
				const startRequest = {
					action: 'start',
					config: {
						host: connection.host,
						port: connection.port,
						user: connection.user,
						pass: connection.password,
						database: connection.database,
						readonly: connection.readonly
					}
				};

				// Start the process
				const process = child_process.spawn(pluginPath, [], {
					stdio: ['pipe', 'pipe', 'pipe']
				});

				// Write start request to stdin
				process.stdin?.write(JSON.stringify(startRequest) + '\n');
				process.stdin?.end();

				// Read response from stdout
				let response = '';
				let responseReceived = false;

				// Show connecting status
				const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
				statusBarItem.text = '$(sync~spin) Connecting to PostgreSQL...';
				statusBarItem.show();

				// Wait for response
				await new Promise<void>((resolve, reject) => {
					// Handle stdout data
					process?.stdout?.on('data', (data) => {
						response += data.toString();
						// Try to parse response as soon as we get data
						try {
							const result = JSON.parse(response);
							if (result.status === 'ready') {
								const port = result.web_port;
								// Store connection info
								connectionInfo[connection.id] = {
									port,
									process
								};
								// Update connection status to connected
								connectionStatus[connection.id] = 'connected';
								treeDataProvider.reload();
								// Open URL in VS Code webview
					const url = `http://localhost:${port}`;
					const panel = vscode.window.createWebviewPanel(
						'shield-postgresql.connection',
						`PostgreSQL - ${connection.name}`,
						vscode.ViewColumn.One,
						{
							enableScripts: true,
							enableCommandUris: true,
							localResourceRoots: [vscode.Uri.joinPath(context.extensionUri, 'src')]
						}
					);
					panel.webview.html = `
						<!DOCTYPE html>
						<html lang="en">
						<head>
							<meta charset="UTF-8">
							<meta name="viewport" content="width=device-width, initial-scale=1.0">
							<title>PostgreSQL Connection</title>
							<style>
								body {
									margin: 0;
									padding: 0;
									overflow: hidden;
								}
								iframe {
									width: 100%;
									height: 100vh;
									border: none;
								}
							</style>
						</head>
						<body>
							<iframe src="${url}"></iframe>
						</body>
						</html>
					`;
								responseReceived = true;
								statusBarItem.text = '$(database) PostgreSQL Connected';
								statusBarItem.show();
								setTimeout(() => statusBarItem.hide(), 3000);
								resolve();
							} else {
								vscode.window.showErrorMessage(`Failed to connect: ${result.message}`);
								// Update connection status to disconnected
								connectionStatus[connection.id] = 'disconnected';
								treeDataProvider.reload();
								responseReceived = true;
								statusBarItem.text = '$(x) PostgreSQL Connection Failed';
								statusBarItem.show();
								setTimeout(() => statusBarItem.hide(), 3000);
								reject(new Error(result.message));
							}
						} catch (e) {
							// Ignore parse errors until we have complete response
						}
					});

					// Handle process errors
					process?.stderr?.on('data', (data) => {
						console.error('shield-plugin-postgres error:', data.toString());
					});

					// Handle process exit
					process?.on('exit', (code) => {
						console.log('shield-plugin-postgres exited with code:', code);
						if (!responseReceived) {
							reject(new Error(`shield-plugin-postgres exited with code ${code} without sending a response`));
							statusBarItem.text = '$(x) PostgreSQL Connection Failed';
							statusBarItem.show();
							setTimeout(() => statusBarItem.hide(), 3000);
						}
						// Update connection status if process exits
						if (connectionStatus[connection.id] === 'connected') {
							connectionStatus[connection.id] = 'disconnected';
							treeDataProvider.reload();
						}
						// Remove connection info
						delete connectionInfo[connection.id];
					});

					// Timeout after 10 seconds
					setTimeout(() => {
						reject(new Error('Timeout waiting for shield-plugin-postgres response'));
						statusBarItem.text = '$(x) PostgreSQL Connection Timeout';
						statusBarItem.show();
						setTimeout(() => statusBarItem.hide(), 3000);
					}, 10000);
				});
			} catch (error) {
				console.error('Error starting shield-plugin-postgres:', error);
				vscode.window.showErrorMessage(`Error starting PostgreSQL client: ${error instanceof Error ? error.message : String(error)}`);
				// Update connection status to disconnected
				connectionStatus[connection.id] = 'disconnected';
				treeDataProvider.reload();
			}
		}
	});

	// Add command to disconnect from tree item
	const disconnectDisposable = vscode.commands.registerCommand('shield-postgresql.disconnect', async (item: ConnectionItem) => {
		if (item) {
			const connection = item.connection;

			// Check if connection is established
			if (connectionStatus[connection.id] === 'connected' && connectionInfo[connection.id]) {
				// Kill the process
				connectionInfo[connection.id].process.kill();
				// Remove connection info
				delete connectionInfo[connection.id];
				// Update connection status
				connectionStatus[connection.id] = 'disconnected';
				treeDataProvider.reload();
				vscode.window.showInformationMessage(`Connection "${connection.name}" disconnected successfully`);
			}
		}
	});

	// Add command to open connection in browser
	const openInBrowserDisposable = vscode.commands.registerCommand('shield-postgresql.openInBrowser', async (item: ConnectionItem) => {
		if (item) {
			const connection = item.connection;

			// Check if connection is established
			if (connectionStatus[connection.id] === 'connected' && connectionInfo[connection.id]) {
				// Open URL in external browser
				const url = `http://localhost:${connectionInfo[connection.id].port}`;
				vscode.env.openExternal(vscode.Uri.parse(url));
			} else {
				// Connection not established, connect first with external browser
				// Update connection status to connecting
				connectionStatus[connection.id] = 'connecting';
				treeDataProvider.reload();

				// Get platform-specific executable name
				const platform = process.platform;
				let executableName = 'shield-plugin-postgres';
				if (platform === 'win32') {
					executableName += '.exe';
				} else if (platform === 'linux') {
					executableName += '-linux';
				}

				// Check if shield-plugin-postgres executable exists
				const pluginPath = path.join(__dirname, '..', 'bin', executableName);
				if (!fs.existsSync(pluginPath)) {
					vscode.window.showErrorMessage('shield-plugin-postgres executable not found for your platform. Please contact the plugin author.');
					// Update connection status to disconnected
					connectionStatus[connection.id] = 'disconnected';
					treeDataProvider.reload();
					return;
				}

				// Stop existing process for this connection if running
				if (connectionInfo[connection.id]) {
					connectionInfo[connection.id].process.kill();
					delete connectionInfo[connection.id];
				}

				// Start shield-plugin-postgres process
				try {
					// Create start request
					const startRequest = {
						action: 'start',
						config: {
							host: connection.host,
							port: connection.port,
							user: connection.user,
							pass: connection.password,
							database: connection.database,
							readonly: connection.readonly
						}
					};

					// Start the process
					const process = child_process.spawn(pluginPath, [], {
						stdio: ['pipe', 'pipe', 'pipe']
					});

					// Write start request to stdin
					process.stdin?.write(JSON.stringify(startRequest) + '\n');
					process.stdin?.end();

					// Read response from stdout
					let response = '';
					let responseReceived = false;

					// Show connecting status
					const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
					statusBarItem.text = '$(sync~spin) Connecting to PostgreSQL...';
					statusBarItem.show();

					// Wait for response
					await new Promise<void>((resolve, reject) => {
						// Handle stdout data
						process?.stdout?.on('data', (data) => {
							response += data.toString();
							// Try to parse response as soon as we get data
							try {
								const result = JSON.parse(response);
								if (result.status === 'ready') {
									const port = result.web_port;
									// Store connection info
									connectionInfo[connection.id] = {
										port,
										process
									};
									// Update connection status to connected
									connectionStatus[connection.id] = 'connected';
									treeDataProvider.reload();
									// Open URL in external browser
									const url = `http://localhost:${port}`;
									vscode.env.openExternal(vscode.Uri.parse(url));
									responseReceived = true;
									statusBarItem.text = '$(database) PostgreSQL Connected';
									statusBarItem.show();
									setTimeout(() => statusBarItem.hide(), 3000);
									resolve();
								} else if (result.status === 'error') {
									console.error('shield-plugin-postgres error:', result.message);
									vscode.window.showErrorMessage(`PostgreSQL connection error: ${result.message}`);
									// Update connection status to disconnected
									connectionStatus[connection.id] = 'disconnected';
									treeDataProvider.reload();
									statusBarItem.text = '$(x) PostgreSQL Connection Failed';
									statusBarItem.show();
									setTimeout(() => statusBarItem.hide(), 3000);
									reject(new Error(result.message));
								}
							} catch (e) {
								// Ignore parse errors until we have complete response
							}
						});

						// Handle process errors
						process?.stderr?.on('data', (data) => {
							console.error('shield-plugin-postgres error:', data.toString());
						});

						// Handle process exit
						process?.on('exit', (code) => {
							console.log('shield-plugin-postgres exited with code:', code);
							if (!responseReceived) {
								reject(new Error(`shield-plugin-postgres exited with code ${code} without sending a response`));
								statusBarItem.text = '$(x) PostgreSQL Connection Failed';
								statusBarItem.show();
								setTimeout(() => statusBarItem.hide(), 3000);
							}
							// Update connection status if process exits
							if (connectionStatus[connection.id] === 'connected') {
								connectionStatus[connection.id] = 'disconnected';
								treeDataProvider.reload();
							}
							// Remove connection info
							delete connectionInfo[connection.id];
						});

						// Timeout after 10 seconds
						setTimeout(() => {
							reject(new Error('Timeout waiting for shield-plugin-postgres response'));
							statusBarItem.text = '$(x) PostgreSQL Connection Timeout';
							statusBarItem.show();
							setTimeout(() => statusBarItem.hide(), 3000);
						}, 10000);
					});
				} catch (error) {
					console.error('Error starting shield-plugin-postgres:', error);
					vscode.window.showErrorMessage(`Error starting PostgreSQL client: ${error instanceof Error ? error.message : String(error)}`);
					// Update connection status to disconnected
					connectionStatus[connection.id] = 'disconnected';
					treeDataProvider.reload();
				}
			}
		}
	});

	// Add command to test connection
	const testConnectionDisposable = vscode.commands.registerCommand('shield-postgresql.testConnection', async (item: ConnectionItem) => {
		if (item) {
			const connection = item.connection;

			// Get platform-specific executable name
			const platform = process.platform;
			let executableName = 'shield-plugin-postgres';
			if (platform === 'win32') {
				executableName += '.exe';
			} else if (platform === 'linux') {
				executableName += '-linux';
			}

			// Check if shield-plugin-postgres executable exists
			const pluginPath = path.join(__dirname, '..', 'bin', executableName);
			if (!fs.existsSync(pluginPath)) {
				vscode.window.showErrorMessage('shield-plugin-postgres executable not found for your platform. Please contact the plugin author.');
				return;
			}

			// Start shield-plugin-postgres process
			try {
				// Create start request
				const startRequest = {
					action: 'start',
					config: {
						host: connection.host,
						port: connection.port,
						user: connection.user,
						pass: connection.password,
						database: connection.database,
						readonly: connection.readonly
					}
				};

				// Start the process
				const testProcess = child_process.spawn(pluginPath, [], {
					stdio: ['pipe', 'pipe', 'pipe']
				});

				// Write start request to stdin
				testProcess.stdin?.write(JSON.stringify(startRequest) + '\n');
				testProcess.stdin?.end();

				// Read response from stdout
				let response = '';
				let responseReceived = false;

				// Show testing status
				const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left);
				statusBarItem.text = '$(sync~spin) Testing connection...';
				statusBarItem.show();

				// Wait for response
				await new Promise<void>((resolve, reject) => {
					// Handle stdout data
					testProcess.stdout?.on('data', (data) => {
						response += data.toString();
						// Try to parse response as soon as we get data
						try {
							const result = JSON.parse(response);
							if (result.status === 'ready') {
								responseReceived = true;
								statusBarItem.text = '$(check) Connection Test Successful';
								statusBarItem.show();
								setTimeout(() => statusBarItem.hide(), 3000);
								vscode.window.showInformationMessage(`Connection to "${connection.name}" is successful!`);
								// Kill the test process
								testProcess.kill();
								resolve();
							} else {
								responseReceived = true;
								statusBarItem.text = '$(x) Connection Test Failed';
								statusBarItem.show();
								setTimeout(() => statusBarItem.hide(), 3000);
								vscode.window.showErrorMessage(`Connection test failed: ${result.message}`);
								testProcess.kill();
								reject(new Error(result.message));
							}
						} catch (e) {
							// Ignore parse errors until we have complete response
						}
					});

					// Handle process errors
					testProcess.stderr?.on('data', (data) => {
						console.error('shield-plugin-postgres error:', data.toString());
					});

					// Handle process exit
					testProcess.on('exit', (code) => {
						console.log('shield-plugin-postgres exited with code:', code);
						if (!responseReceived) {
							reject(new Error(`shield-plugin-postgres exited with code ${code} without sending a response`));
							statusBarItem.text = '$(x) Connection Test Failed';
							statusBarItem.show();
							setTimeout(() => statusBarItem.hide(), 3000);
						}
					});

					// Timeout after 10 seconds
					setTimeout(() => {
						reject(new Error('Timeout waiting for shield-plugin-postgres response'));
						statusBarItem.text = '$(x) Connection Test Timeout';
						statusBarItem.show();
						setTimeout(() => statusBarItem.hide(), 3000);
						testProcess.kill();
					}, 10000);
				});
			} catch (error) {
				console.error('Error testing connection:', error);
				vscode.window.showErrorMessage(`Error testing connection: ${error instanceof Error ? error.message : String(error)}`);
			}
		}
	});

	// Add subscriptions
	context.subscriptions.push(
		connectDisposable,
		addConnectionDisposable,
		removeConnectionDisposable,
		editConnectionDisposable,
		connectFromTreeDisposable,
		testConnectionDisposable
	);
}

// Open PostgreSQL Web Client in browser
function openPostgreSQLWebView(context: vscode.ExtensionContext, port: number) {
	// Construct the URL
	const url = `http://localhost:${port}`;
	
	// Open the URL in the default browser
	vscode.env.openExternal(vscode.Uri.parse(url)).then(
		(success) => {
			if (success) {
				console.log(`Opened PostgreSQL Web Client in browser: ${url}`);
			} else {
				console.error('Failed to open browser');
				vscode.window.showErrorMessage('Failed to open browser. Please open http://localhost:' + port + ' manually.');
			}
		},
		(error) => {
			console.error('Error opening browser:', error);
			vscode.window.showErrorMessage('Error opening browser: ' + error.message);
		}
	);
}

// This method is called when your extension is deactivated
export function deactivate() {
	// Stop the process when the extension is deactivated
	if (postgresProcess) {
		postgresProcess.kill();
		postgresProcess = null;
		webPort = null;
	}
}
