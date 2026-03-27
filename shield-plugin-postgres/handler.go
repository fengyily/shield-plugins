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

// erHandler returns all tables with columns and foreign key relationships for ER diagram.
// Query param: schema (default: public)
func erHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		schema := r.URL.Query().Get("schema")
		if schema == "" {
			schema = "public"
		}

		// Get all tables with their columns
		colRows, err := db.Query(`
			SELECT c.table_name, c.column_name, c.data_type,
				CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END AS is_pk
			FROM information_schema.columns c
			LEFT JOIN (
				SELECT kcu.table_name, kcu.column_name
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_schema = $1
			) pk ON c.table_name = pk.table_name AND c.column_name = pk.column_name
			WHERE c.table_schema = $1
			ORDER BY c.table_name, c.ordinal_position`, schema)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer colRows.Close()

		type ERColumn struct {
			Name string `json:"name"`
			Type string `json:"type"`
			PK   bool   `json:"pk"`
		}
		type ERTable struct {
			Name    string     `json:"name"`
			Columns []ERColumn `json:"columns"`
		}

		tableMap := make(map[string]*ERTable)
		var tableOrder []string

		for colRows.Next() {
			var tbl, col, dtype string
			var pk bool
			colRows.Scan(&tbl, &col, &dtype, &pk)

			t, ok := tableMap[tbl]
			if !ok {
				t = &ERTable{Name: tbl}
				tableMap[tbl] = t
				tableOrder = append(tableOrder, tbl)
			}
			t.Columns = append(t.Columns, ERColumn{Name: col, Type: dtype, PK: pk})
		}

		var tables []ERTable
		for _, name := range tableOrder {
			tables = append(tables, *tableMap[name])
		}

		// Get foreign key relationships
		fkRows, err := db.Query(`
			SELECT
				tc.constraint_name,
				kcu.table_name AS from_table,
				kcu.column_name AS from_column,
				ccu.table_name AS to_table,
				ccu.column_name AS to_column
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage ccu
				ON tc.constraint_name = ccu.constraint_name
				AND tc.table_schema = ccu.table_schema
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = $1
			ORDER BY kcu.table_name, kcu.column_name`, schema)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer fkRows.Close()

		type ERRelation struct {
			Constraint string `json:"constraint"`
			FromTable  string `json:"from_table"`
			FromColumn string `json:"from_column"`
			ToTable    string `json:"to_table"`
			ToColumn   string `json:"to_column"`
		}

		var relations []ERRelation
		for fkRows.Next() {
			var rel ERRelation
			fkRows.Scan(&rel.Constraint, &rel.FromTable, &rel.FromColumn, &rel.ToTable, &rel.ToColumn)
			relations = append(relations, rel)
		}

		writeJSON(w, 200, map[string]interface{}{
			"tables":    tables,
			"relations": relations,
		})
	}
}

// exportSQLHandler exports table structure and optionally data as SQL.
// Query params: schema, table, mode (structure|all)
// exportTableSQL exports a single table's DDL (and optionally data) into sb.
func exportTableSQL(db *sql.DB, schema, table, mode string, sb *strings.Builder) error {
	sb.WriteString("\n-- Table: " + quoteIdentifier(schema) + "." + quoteIdentifier(table) + "\n")

	// ── Column definitions ──
	colRows, err := db.Query(`
		SELECT column_name, data_type, character_maximum_length,
			numeric_precision, numeric_scale, is_nullable, column_default,
			udt_name
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return err
	}
	defer colRows.Close()

	type colDef struct {
		name, fullType string
		nullable       bool
		defaultVal     *string
		isSerial       bool
	}
	var cols []colDef

	for colRows.Next() {
		var name, dataType, isNullable, udtName string
		var charMaxLen, numPrec, numScale *int
		var colDefault *string
		colRows.Scan(&name, &dataType, &charMaxLen, &numPrec, &numScale, &isNullable, &colDefault, &udtName)

		fullType := resolveColumnType(dataType, udtName, charMaxLen, numPrec, numScale, colDefault)
		serial := colDefault != nil && strings.Contains(*colDefault, "nextval(")
		cols = append(cols, colDef{
			name:       name,
			fullType:   fullType,
			nullable:   isNullable == "YES",
			defaultVal: colDefault,
			isSerial:   serial,
		})
	}

	if len(cols) == 0 {
		return nil
	}

	// ── Primary key ──
	pkRows, err := db.Query(`
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY kcu.ordinal_position`, schema, table)
	if err != nil {
		return err
	}
	defer pkRows.Close()
	var pkCols []string
	for pkRows.Next() {
		var col string
		pkRows.Scan(&col)
		pkCols = append(pkCols, col)
	}

	// ── Unique constraints ──
	uqRows, err := db.Query(`
		SELECT tc.constraint_name,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position)
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'UNIQUE'
			AND tc.table_schema = $1 AND tc.table_name = $2
		GROUP BY tc.constraint_name
		ORDER BY tc.constraint_name`, schema, table)
	if err != nil {
		return err
	}
	defer uqRows.Close()
	type uqDef struct {
		name string
		cols []string
	}
	var uqConstraints []uqDef
	for uqRows.Next() {
		var name string
		var colsArr []byte
		uqRows.Scan(&name, &colsArr)
		parsed := parsePostgresArray(string(colsArr))
		uqConstraints = append(uqConstraints, uqDef{name: name, cols: parsed})
	}

	// ── Foreign keys ──
	fkRows, err := db.Query(`
		SELECT tc.constraint_name,
			kcu.column_name,
			ccu.table_schema AS ref_schema,
			ccu.table_name AS ref_table,
			ccu.column_name AS ref_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY tc.constraint_name`, schema, table)
	if err != nil {
		return err
	}
	defer fkRows.Close()
	type fkDef struct {
		name, col, refSchema, refTable, refCol string
	}
	var fkConstraints []fkDef
	for fkRows.Next() {
		var fk fkDef
		fkRows.Scan(&fk.name, &fk.col, &fk.refSchema, &fk.refTable, &fk.refCol)
		fkConstraints = append(fkConstraints, fk)
	}

	// ── Build CREATE TABLE ──
	sb.WriteString("CREATE TABLE " + quoteIdentifier(schema) + "." + quoteIdentifier(table) + " (\n")
	for i, col := range cols {
		sb.WriteString("    " + quoteIdentifier(col.name) + " " + col.fullType)
		if !col.nullable {
			sb.WriteString(" NOT NULL")
		}
		if !col.isSerial && col.defaultVal != nil && *col.defaultVal != "" {
			sb.WriteString(" DEFAULT " + *col.defaultVal)
		}
		if i < len(cols)-1 || len(pkCols) > 0 || len(uqConstraints) > 0 || len(fkConstraints) > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	trailingItems := 0
	if len(pkCols) > 0 {
		trailingItems++
	}
	trailingItems += len(uqConstraints)
	trailingItems += len(fkConstraints)
	trailing := 0

	if len(pkCols) > 0 {
		trailing++
		quoted := make([]string, len(pkCols))
		for i, c := range pkCols {
			quoted[i] = quoteIdentifier(c)
		}
		sb.WriteString("    PRIMARY KEY (" + strings.Join(quoted, ", ") + ")")
		if trailing < trailingItems {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	for _, uq := range uqConstraints {
		trailing++
		quoted := make([]string, len(uq.cols))
		for i, c := range uq.cols {
			quoted[i] = quoteIdentifier(c)
		}
		sb.WriteString("    CONSTRAINT " + quoteIdentifier(uq.name) + " UNIQUE (" + strings.Join(quoted, ", ") + ")")
		if trailing < trailingItems {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	for fi, fk := range fkConstraints {
		sb.WriteString("    CONSTRAINT " + quoteIdentifier(fk.name) +
			" FOREIGN KEY (" + quoteIdentifier(fk.col) + ") REFERENCES " +
			quoteIdentifier(fk.refSchema) + "." + quoteIdentifier(fk.refTable) +
			"(" + quoteIdentifier(fk.refCol) + ")")
		if fi < len(fkConstraints)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(");\n")

	// ── Indexes (non-PK, non-unique-constraint) ──
	idxRows, err := db.Query(`
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = $1 AND tablename = $2
			AND indexname NOT IN (
				SELECT constraint_name FROM information_schema.table_constraints
				WHERE table_schema = $1 AND table_name = $2
			)`, schema, table)
	if err == nil {
		defer idxRows.Close()
		for idxRows.Next() {
			var def string
			idxRows.Scan(&def)
			sb.WriteString("\n" + def + ";\n")
		}
	}

	// ── Data export ──
	if mode == "all" {
		sb.WriteString("\n")

		dataRows, err := db.Query(fmt.Sprintf("SELECT * FROM %s.%s",
			quoteIdentifier(schema), quoteIdentifier(table)))
		if err != nil {
			return err
		}
		defer dataRows.Close()

		dataCols, _ := dataRows.Columns()
		colTypes, _ := dataRows.ColumnTypes()

		for dataRows.Next() {
			values := make([]interface{}, len(dataCols))
			valuePtrs := make([]interface{}, len(dataCols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			dataRows.Scan(valuePtrs...)

			sb.WriteString("INSERT INTO " + quoteIdentifier(schema) + "." + quoteIdentifier(table) + " (")
			for i, c := range dataCols {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(quoteIdentifier(c))
			}
			sb.WriteString(") VALUES (")

			for i, val := range values {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(formatSQLValue(val, colTypes[i]))
			}
			sb.WriteString(");\n")
		}
	}

	return nil
}

// exportSQLHandler exports table structure and optionally data as SQL.
// Query params: schema, table (optional — omit for whole schema), mode (structure|all)
func exportSQLHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		schema := r.URL.Query().Get("schema")
		table := r.URL.Query().Get("table")
		mode := r.URL.Query().Get("mode")
		if schema == "" {
			schema = "public"
		}
		if mode == "" {
			mode = "structure"
		}

		var sb strings.Builder

		if table != "" {
			// ── Single table export ──
			sb.WriteString("-- Table: " + quoteIdentifier(schema) + "." + quoteIdentifier(table) + "\n")
			sb.WriteString("-- Generated by Shield PostgreSQL\n")
			if err := exportTableSQL(db, schema, table, mode, &sb); err != nil {
				writeError(w, 500, err.Error())
				return
			}
		} else {
			// ── Whole schema export ──
			sb.WriteString("-- Schema: " + quoteIdentifier(schema) + "\n")
			sb.WriteString("-- Generated by Shield PostgreSQL\n")
			sb.WriteString("-- Mode: " + mode + "\n\n")

			tableRows, err := db.Query(`
				SELECT table_name
				FROM information_schema.tables
				WHERE table_schema = $1 AND table_type = 'BASE TABLE'
				ORDER BY table_name`, schema)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			defer tableRows.Close()

			var tables []string
			for tableRows.Next() {
				var t string
				tableRows.Scan(&t)
				tables = append(tables, t)
			}

			if len(tables) == 0 {
				writeError(w, 404, "no tables found in schema")
				return
			}

			// First pass: CREATE TABLEs (structure only, no data yet)
			for _, t := range tables {
				if err := exportTableSQL(db, schema, t, "structure", &sb); err != nil {
					writeError(w, 500, err.Error())
					return
				}
				sb.WriteString("\n")
			}

			// Second pass: INSERT data (after all tables are created, so FK references are valid)
			if mode == "all" {
				sb.WriteString("\n-- ══════════════════════════════════════════════\n")
				sb.WriteString("--  Data\n")
				sb.WriteString("-- ══════════════════════════════════════════════\n")
				for _, t := range tables {
					if err := exportTableData(db, schema, t, &sb); err != nil {
						writeError(w, 500, err.Error())
						return
					}
				}
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(sb.String()))
	}
}

// exportTableData exports INSERT statements for a single table.
func exportTableData(db *sql.DB, schema, table string, sb *strings.Builder) error {
	dataRows, err := db.Query(fmt.Sprintf("SELECT * FROM %s.%s",
		quoteIdentifier(schema), quoteIdentifier(table)))
	if err != nil {
		return err
	}
	defer dataRows.Close()

	dataCols, _ := dataRows.Columns()
	colTypes, _ := dataRows.ColumnTypes()
	hasData := false

	for dataRows.Next() {
		if !hasData {
			sb.WriteString("\n-- Data for: " + quoteIdentifier(schema) + "." + quoteIdentifier(table) + "\n")
			hasData = true
		}
		values := make([]interface{}, len(dataCols))
		valuePtrs := make([]interface{}, len(dataCols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		dataRows.Scan(valuePtrs...)

		sb.WriteString("INSERT INTO " + quoteIdentifier(schema) + "." + quoteIdentifier(table) + " (")
		for i, c := range dataCols {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(quoteIdentifier(c))
		}
		sb.WriteString(") VALUES (")

		for i, val := range values {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(formatSQLValue(val, colTypes[i]))
		}
		sb.WriteString(");\n")
	}
	return nil
}

// resolveColumnType maps information_schema types to SQL DDL types.
func resolveColumnType(dataType, udtName string, charMaxLen, numPrec, numScale *int, colDefault *string) string {
	// Check for serial types via default value pattern
	isSerial := colDefault != nil && strings.Contains(*colDefault, "nextval(")
	switch dataType {
	case "integer":
		if isSerial {
			return "SERIAL"
		}
		return "INTEGER"
	case "bigint":
		if isSerial {
			return "BIGSERIAL"
		}
		return "BIGINT"
	case "smallint":
		if isSerial {
			return "SMALLSERIAL"
		}
		return "SMALLINT"
	case "character varying":
		if charMaxLen != nil {
			return fmt.Sprintf("VARCHAR(%d)", *charMaxLen)
		}
		return "VARCHAR"
	case "character":
		if charMaxLen != nil {
			return fmt.Sprintf("CHAR(%d)", *charMaxLen)
		}
		return "CHAR"
	case "numeric":
		if numPrec != nil && numScale != nil {
			return fmt.Sprintf("NUMERIC(%d,%d)", *numPrec, *numScale)
		}
		if numPrec != nil {
			return fmt.Sprintf("NUMERIC(%d)", *numPrec)
		}
		return "NUMERIC"
	case "ARRAY":
		return udtName // e.g. _text -> text[]
	case "USER-DEFINED":
		return udtName
	default:
		return strings.ToUpper(dataType)
	}
}

// formatSQLValue converts a Go value to SQL literal.
func formatSQLValue(val interface{}, ct *sql.ColumnType) string {
	if val == nil {
		return "NULL"
	}
	switch v := val.(type) {
	case []byte:
		s := string(v)
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'"
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	case int64, int32, int16, int, float64, float32:
		return fmt.Sprintf("%v", v)
	case time.Time:
		return "'" + v.Format("2006-01-02 15:04:05.999999-07") + "'"
	default:
		s := fmt.Sprintf("%v", v)
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
}

// parsePostgresArray parses a simple PostgreSQL text array like {a,b,c}.
func parsePostgresArray(s string) []string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// quoteIdentifier wraps a PostgreSQL identifier in double quotes.
func quoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
