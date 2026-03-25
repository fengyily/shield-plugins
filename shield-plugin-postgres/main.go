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

func respond(resp StartResponse) {
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	respond(StartResponse{Status: "error", Message: msg})
}

func handleStart(cfg PluginConfig) {
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
		respondError(fmt.Sprintf("failed to open connection: %v", err))
		return
	}
	if err := db.Ping(); err != nil {
		respondError(fmt.Sprintf("cannot connect to PostgreSQL at %s:%d: %v", cfg.Host, cfg.Port, err))
		return
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		respondError(fmt.Sprintf("failed to find available port: %v", err))
		return
	}
	webPort := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()

	mux.HandleFunc("/api/schemas", schemasHandler(db))
	mux.HandleFunc("/api/tables", tablesHandler(db))
	mux.HandleFunc("/api/columns", columnsHandler(db))
	mux.HandleFunc("/api/indexes", indexesHandler(db))
	mux.HandleFunc("/api/query", queryHandler(db, cfg.ReadOnly))
	mux.HandleFunc("/api/info", infoHandler(db, cfg))

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

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
