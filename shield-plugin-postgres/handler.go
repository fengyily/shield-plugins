package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiResponse{Code: code, Data: data})
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(apiResponse{Code: code, Message: msg})
}

// infoHandler returns server info.
func infoHandler(db *sql.DB, cfg PluginConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var version string
		db.QueryRow("SELECT version()").Scan(&version)

		writeJSON(w, 200, map[string]interface{}{
			"host":     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			"user":     cfg.User,
			"database": cfg.Database,
			"version":  version,
			"readonly": cfg.ReadOnly,
		})
	}
}

// schemasHandler returns list of schemas (excluding system schemas).
func schemasHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT schema_name
			FROM information_schema.schemata
			WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
			ORDER BY schema_name`)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		var schemas []string
		for rows.Next() {
			var name string
			rows.Scan(&name)
			schemas = append(schemas, name)
		}
		writeJSON(w, 200, schemas)
	}
}

// tablesHandler returns list of tables for a schema.
// Query param: schema (default: public)
func tablesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		schema := r.URL.Query().Get("schema")
		if schema == "" {
			schema = "public"
		}

		rows, err := db.Query(`
			SELECT table_name
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_type = 'BASE TABLE'
			ORDER BY table_name`, schema)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		var tables []string
		for rows.Next() {
			var name string
			rows.Scan(&name)
			tables = append(tables, name)
		}
		writeJSON(w, 200, tables)
	}
}

// columnsHandler returns column info for a table.
// Query params: schema, table
func columnsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		schema := r.URL.Query().Get("schema")
		table := r.URL.Query().Get("table")
		if schema == "" {
			schema = "public"
		}
		if table == "" {
			writeError(w, 400, "table parameter is required")
			return
		}

		rows, err := db.Query(`
			SELECT
				c.column_name,
				c.data_type,
				COALESCE(c.character_maximum_length::text, c.numeric_precision::text, ''),
				c.is_nullable,
				COALESCE(c.column_default, ''),
				CASE
					WHEN pk.column_name IS NOT NULL THEN 'PRI'
					WHEN uq.column_name IS NOT NULL THEN 'UNI'
					ELSE ''
				END AS key_type
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT kcu.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
					AND tc.table_schema = $1
					AND tc.table_name = $2
			) pk ON c.column_name = pk.column_name
			LEFT JOIN (
				SELECT kcu.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'UNIQUE'
					AND tc.table_schema = $1
					AND tc.table_name = $2
			) uq ON c.column_name = uq.column_name
			WHERE c.table_schema = $1 AND c.table_name = $2
			ORDER BY c.ordinal_position`,
			schema, table)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		type Column struct {
			Field    string `json:"field"`
			Type     string `json:"type"`
			Length   string `json:"length"`
			Nullable string `json:"null"`
			Default  string `json:"default"`
			Key      string `json:"key"`
		}

		var columns []Column
		for rows.Next() {
			var col Column
			rows.Scan(&col.Field, &col.Type, &col.Length, &col.Nullable, &col.Default, &col.Key)
			columns = append(columns, col)
		}
		writeJSON(w, 200, columns)
	}
}

// indexesHandler returns index info for a table.
// Query params: schema, table
func indexesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		schema := r.URL.Query().Get("schema")
		table := r.URL.Query().Get("table")
		if schema == "" {
			schema = "public"
		}
		if table == "" {
			writeError(w, 400, "table parameter is required")
			return
		}

		rows, err := db.Query(`
			SELECT
				i.relname AS index_name,
				am.amname AS index_type,
				ix.indisunique AS is_unique,
				ix.indisprimary AS is_primary,
				array_to_string(ARRAY(
					SELECT a.attname
					FROM unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord)
					JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
					ORDER BY k.ord
				), ',') AS columns
			FROM pg_index ix
			JOIN pg_class t ON t.oid = ix.indrelid
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN pg_am am ON am.oid = i.relam
			JOIN pg_namespace n ON n.oid = t.relnamespace
			WHERE n.nspname = $1 AND t.relname = $2
			ORDER BY i.relname`,
			schema, table)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		type Index struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			Unique    bool   `json:"unique"`
			Primary   bool   `json:"primary"`
			Columns   string `json:"columns"`
		}

		var indexes []Index
		for rows.Next() {
			var idx Index
			rows.Scan(&idx.Name, &idx.Type, &idx.Unique, &idx.Primary, &idx.Columns)
			indexes = append(indexes, idx)
		}
		writeJSON(w, 200, indexes)
	}
}

// queryHandler executes a SQL query and returns results.
// POST body: {"sql": "SELECT ...", "schema": "public"}
func queryHandler(db *sql.DB, readOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, 405, "POST required")
			return
		}

		var req struct {
			SQL    string `json:"sql"`
			Schema string `json:"schema"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "invalid request body")
			return
		}

		if strings.TrimSpace(req.SQL) == "" {
			writeError(w, 400, "sql is required")
			return
		}

		if readOnly && isWriteSQL(req.SQL) {
			writeError(w, 403, "blocked: read-only mode is enabled")
			return
		}

		// Switch schema if specified
		if req.Schema != "" {
			if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s", quoteIdentifier(req.Schema))); err != nil {
				writeError(w, 500, fmt.Sprintf("failed to switch schema: %v", err))
				return
			}
		}

		start := time.Now()

		trimmed := strings.TrimSpace(strings.ToUpper(req.SQL))
		isQuery := strings.HasPrefix(trimmed, "SELECT") ||
			strings.HasPrefix(trimmed, "SHOW") ||
			strings.HasPrefix(trimmed, "EXPLAIN") ||
			strings.HasPrefix(trimmed, "WITH") ||
			strings.HasPrefix(trimmed, "TABLE") ||
			strings.HasPrefix(trimmed, "VALUES")

		if isQuery {
			rows, err := db.Query(req.SQL)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			defer rows.Close()

			columns, _ := rows.Columns()
			var results []map[string]interface{}

			for rows.Next() {
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range values {
					valuePtrs[i] = &values[i]
				}
				rows.Scan(valuePtrs...)

				row := make(map[string]interface{})
				for i, col := range columns {
					val := values[i]
					if b, ok := val.([]byte); ok {
						row[col] = string(b)
					} else {
						row[col] = val
					}
				}
				results = append(results, row)
			}

			duration := time.Since(start)
			writeJSON(w, 200, map[string]interface{}{
				"columns":  columns,
				"rows":     results,
				"count":    len(results),
				"duration": duration.String(),
			})
		} else {
			result, err := db.Exec(req.SQL)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}

			affected, _ := result.RowsAffected()
			duration := time.Since(start)

			writeJSON(w, 200, map[string]interface{}{
				"affected_rows": affected,
				"duration":      duration.String(),
			})
		}
	}
}

// isWriteSQL checks if a SQL statement is a write operation.
func isWriteSQL(sql string) bool {
	t := strings.TrimSpace(strings.ToUpper(sql))
	writeKeywords := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "TRUNCATE", "GRANT", "REVOKE"}
	for _, k := range writeKeywords {
		if strings.HasPrefix(t, k) {
			return true
		}
	}
	return false
}

// quoteIdentifier wraps a PostgreSQL identifier in double quotes.
func quoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
