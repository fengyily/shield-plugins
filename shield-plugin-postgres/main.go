package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	_ "github.com/lib/pq"
)

//go:embed static/*
var staticFS embed.FS

// Protocol types (matches shield-cli/plugin package)

type StartRequest struct {
	Action string       `json:"action"`
	Config PluginConfig `json:"config,omitempty"`
}

type PluginConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user,omitempty"`
	Pass     string `json:"pass,omitempty"`
	Database string `json:"database,omitempty"`
	ReadOnly bool   `json:"readonly,omitempty"`
}

type StartResponse struct {
	Status  string `json:"status"`
	WebPort int    `json:"web_port,omitempty"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

func main() {
	// Standalone mode: run as Docker container / standalone binary
	if os.Getenv("DB_HOST") != "" {
		standaloneMode()
		return
	}

	// Plugin protocol mode: read JSON commands from stdin
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

func standaloneMode() {
	port := 5432
	if v := os.Getenv("DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}
	readOnly := false
	if v := os.Getenv("DB_READONLY"); v == "true" || v == "1" {
		readOnly = true
	}
	webPort := 8080
	if v := os.Getenv("WEB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			webPort = p
		}
	}

	cfg := PluginConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Pass:     os.Getenv("DB_PASS"),
		Database: os.Getenv("DB_NAME"),
		ReadOnly: readOnly,
	}

	db, err := connectDB(cfg)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	hub := newCollabHub()
	go hub.run()
	mux := setupHTTP(db, cfg, hub)

	addr := fmt.Sprintf("0.0.0.0:%d", webPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	fmt.Fprintf(os.Stderr, "PostgreSQL Web Client listening on %s\n", addr)

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Fprintf(os.Stderr, "Shutting down\n")
}

func respond(resp StartResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	respond(StartResponse{Status: "error", Message: msg})
}

func connectDB(cfg PluginConfig) (*sql.DB, error) {
	user := cfg.User
	if user == "" {
		user = "postgres"
	}
	database := cfg.Database
	if database == "" {
		database = "postgres"
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, user, cfg.Pass, database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot connect to PostgreSQL at %s:%d: %w", cfg.Host, cfg.Port, err)
	}
	return db, nil
}

func setupHTTP(db *sql.DB, cfg PluginConfig, hub *CollabHub) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/schemas", schemasHandler(db))
	mux.HandleFunc("/api/tables", tablesHandler(db))
	mux.HandleFunc("/api/columns", columnsHandler(db))
	mux.HandleFunc("/api/indexes", indexesHandler(db))
	mux.HandleFunc("/api/query", queryHandler(db, cfg.ReadOnly))
	mux.HandleFunc("/api/info", infoHandler(db, cfg))
	mux.HandleFunc("/api/er", erHandler(db))
	mux.HandleFunc("/api/export", exportSQLHandler(db))
	mux.HandleFunc("/ws/er", collabHandler(hub))

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	return mux
}

func handleStart(cfg PluginConfig) {
	db, err := connectDB(cfg)
	if err != nil {
		respondError(err.Error())
		return
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		db.Close()
		respondError(fmt.Sprintf("failed to find available port: %v", err))
		return
	}
	webPort := listener.Addr().(*net.TCPAddr).Port

	hub := newCollabHub()
	go hub.run()
	mux := setupHTTP(db, cfg, hub)

	respond(StartResponse{
		Status:  "ready",
		WebPort: webPort,
		Name:    "PostgreSQL Web Client",
		Version: "0.1.0",
	})

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	db.Close()
}
