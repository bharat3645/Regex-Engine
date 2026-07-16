// File: database/manager.go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"Regex/types"

	_ "github.com/mattn/go-sqlite3"
)

// FileCacheEntry represents the structure of a record in the file_cache table.
type FileCacheEntry struct {
	Path         string
	Checksum     string
	ModifiedTime int64
	LastScanned  time.Time
}

// Manager handles all database operations.
type Manager struct {
	db *sql.DB
}

// DB exposes the underlying SQL DB for complex queries.
func (m *Manager) DB() *sql.DB {
	return m.db
}

// NewManager initializes the database connection and ensures schema is up to date.
func NewManager() (*Manager, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not get executable path: %w", err)
	}
	dbPath := filepath.Join(filepath.Dir(exePath), "dlp_scan_data.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	m := &Manager{db: db}
	if err := m.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return m, nil
}

// initSchema creates the necessary tables if they don't exist.
func (m *Manager) initSchema() error {
	// Table for scan jobs
	_, err := m.db.Exec(`
        CREATE TABLE IF NOT EXISTS jobs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            path TEXT NOT NULL UNIQUE,
            extension TEXT NOT NULL,
            status TEXT NOT NULL DEFAULT 'pending',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );
    `)
	if err != nil {
		return err
	}

	// Table for caching file checksums and metadata
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS file_cache (
			path TEXT PRIMARY KEY,
			checksum TEXT NOT NULL,
			modified_time INTEGER NOT NULL,
			last_scanned DATETIME NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// Key-value table for storing metadata like rules checksum
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS scan_metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// HUB-AND-SPOKE SCHEMA START
	// Hierarchy Nodes: The backbone of the tree structure
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS hierarchy_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			parent_id INTEGER DEFAULT 0,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE
		);
	`)
	if err != nil {
		return err
	}

	// PII Tags: Risk scoring instead of raw content
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS pii_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id INTEGER NOT NULL,
			rule_id TEXT NOT NULL,
			risk_score REAL DEFAULT 0.0,
			count INTEGER DEFAULT 1,
			confidence_score REAL DEFAULT 0.0,
			vector_embedding BLOB,
			validation_status TEXT DEFAULT 'pending',
			pii_type_refined TEXT,
			FOREIGN KEY(node_id) REFERENCES hierarchy_nodes(id)
		);
	`)
	if err != nil {
		return err
	}

	// Migration: Ensure new columns exist for existing databases
	if err := m.ensureColumnExists("pii_tags", "confidence_score", "REAL DEFAULT 0.0"); err != nil {
		fmt.Printf("Migration warning: %v\n", err)
	}
	if err := m.ensureColumnExists("pii_tags", "vector_embedding", "BLOB"); err != nil {
		fmt.Printf("Migration warning: %v\n", err)
	}
	if err := m.ensureColumnExists("pii_tags", "validation_status", "TEXT DEFAULT 'pending'"); err != nil {
		fmt.Printf("Migration warning: %v\n", err)
	}
	if err := m.ensureColumnExists("pii_tags", "pii_type_refined", "TEXT"); err != nil {
		fmt.Printf("Migration warning: %v\n", err)
	}

	// HUB-AND-SPOKE SCHEMA START - GRAPH EDGES
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS node_relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_node_id INTEGER NOT NULL,
			target_node_id INTEGER NOT NULL,
			type TEXT NOT NULL, -- e.g., "CopyOf", "DerivedFrom"
			weight REAL DEFAULT 1.0,
			FOREIGN KEY(source_node_id) REFERENCES hierarchy_nodes(id),
			FOREIGN KEY(target_node_id) REFERENCES hierarchy_nodes(id)
		);
	`)
	// HUB-AND-SPOKE SCHEMA END
	// HUB-AND-SPOKE SCHEMA END

	// OPTIMIZATION: Indexes
	indexQueries := []string{
		"CREATE INDEX IF NOT EXISTS idx_file_cache_checksum ON file_cache(checksum);",
		"CREATE INDEX IF NOT EXISTS idx_hierarchy_nodes_path ON hierarchy_nodes(path);",
		"CREATE INDEX IF NOT EXISTS idx_hierarchy_nodes_parent ON hierarchy_nodes(parent_id);",
		"CREATE INDEX IF NOT EXISTS idx_pii_tags_node ON pii_tags(node_id);",
		"CREATE INDEX IF NOT EXISTS idx_node_relations_source ON node_relations(source_node_id);",
		"CREATE INDEX IF NOT EXISTS idx_node_relations_target ON node_relations(target_node_id);",
	}

	for _, query := range indexQueries {
		if _, err := m.db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

// ensureColumnExists checks if a column exists and adds it if not
func (m *Manager) ensureColumnExists(tableName, columnName, checkType string) error {
	// Check if column exists
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := m.db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	exists := false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			exists = true
			break
		}
	}

	if !exists {
		alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, checkType)
		if _, err := m.db.Exec(alterQuery); err != nil {
			return fmt.Errorf("failed to add column %s: %w", columnName, err)
		}
	}
	return nil
}

// GetFileCacheEntry retrieves a single cache entry for a given file path.
func (m *Manager) GetFileCacheEntry(ctx context.Context, path string) (*FileCacheEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	row := m.db.QueryRowContext(ctx, "SELECT checksum, modified_time FROM file_cache WHERE path = ?", path)
	var entry FileCacheEntry
	entry.Path = path
	err := row.Scan(&entry.Checksum, &entry.ModifiedTime)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is not an error
	}
	return &entry, err
}

// UpdateFileCacheEntry inserts or updates a file's cache information.
func (m *Manager) UpdateFileCacheEntry(ctx context.Context, entry FileCacheEntry) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO file_cache (path, checksum, modified_time, last_scanned)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			checksum = excluded.checksum,
			modified_time = excluded.modified_time,
			last_scanned = excluded.last_scanned;
	`, entry.Path, entry.Checksum, entry.ModifiedTime, time.Now())
	return err
}

// ClearFileCache removes all entries from the file cache. Used when rules change.
func (m *Manager) ClearFileCache(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second) // Longer timeout for bulk delete
	defer cancel()
	_, err := m.db.ExecContext(ctx, "DELETE FROM file_cache")
	return err
}

// GetMetadata retrieves a value from the metadata table.
func (m *Manager) GetMetadata(ctx context.Context, key string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	row := m.db.QueryRowContext(ctx, "SELECT value FROM scan_metadata WHERE key = ?", key)
	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Not found is not an error
	}
	return value, err
}

// SetMetadata sets a value in the metadata table.
func (m *Manager) SetMetadata(ctx context.Context, key, value string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO scan_metadata (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value;
	`, key, value)
	return err
}

// AddJobsBatch inserts multiple jobs into the database.
func (m *Manager) AddJobsBatch(ctx context.Context, jobs []types.FileJob) error {
	if len(jobs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second) // Bulk insert
	defer cancel()

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO jobs (path, extension) VALUES (?, ?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, job := range jobs {
		if _, err := stmt.ExecContext(ctx, job.Path, job.Extension); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetPendingJobsBatch retrieves a batch of pending jobs.
func (m *Manager) GetPendingJobsBatch(ctx context.Context, limit int) ([]types.FileJob, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := m.db.QueryContext(ctx, "SELECT id, path, extension FROM jobs WHERE status = 'pending' LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []types.FileJob
	for rows.Next() {
		var job types.FileJob
		if err := rows.Scan(&job.ID, &job.Path, &job.Extension); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// UpdateJobsStatus updates the status of a batch of jobs.
func (m *Manager) UpdateJobsStatus(ctx context.Context, jobs []types.FileJob, status string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, "UPDATE jobs SET status = ? WHERE id = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, job := range jobs {
		if _, err := stmt.ExecContext(ctx, status, job.ID); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// SaveRiskTags bulk inserts risk tags into the database.
func (m *Manager) SaveRiskTags(ctx context.Context, nodeID int64, tags []types.RiskTag) error {
	if len(tags) == 0 {
		return nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO pii_tags (
			node_id, rule_id, risk_score, count, 
			confidence_score, vector_embedding, validation_status, pii_type_refined
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tag := range tags {
		// Serialize vector embedding (simple JSON or bytes, here using JSON string for simplicity in BLOB)
		// actually, sqlite BLOB can take []byte. We need to serialize []float64 to []byte
		// For simplicity/debugging, let's store as a string if possible, or skip complex serialization for now.
		// Let's use a dummy empty blob for now as the serializer isn't imported.
		// BETTER: Use a helper or just []byte(fmt.Sprintf("%v", tag.VectorEmbedding))
		vectorBlob := []byte(fmt.Sprintf("%v", tag.VectorEmbedding))

		_, err := stmt.ExecContext(ctx,
			nodeID, tag.RuleID, tag.RiskScore, tag.Count,
			tag.ConfidenceScore, vectorBlob, tag.ValidationStatus, tag.PIITypeRefined,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
func (m *Manager) ClearJobs() error {
	_, err := m.db.Exec("DELETE FROM jobs")
	return err
}
func (m *Manager) Close() {
	if m.db != nil {
		m.db.Close()
	}
}

// --- Hierarchy Methods ---

// GetOrCreateNode gets a node ID by path, or creates it if it doesn't exist.
func (m *Manager) GetOrCreateNode(ctx context.Context, parentID int64, name string, nodeType types.NodeType, path string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check if exists
	var id int64
	err := m.db.QueryRowContext(ctx, "SELECT id FROM hierarchy_nodes WHERE path = ?", path).Scan(&id)
	if err == nil {
		return id, nil
	} else if err != sql.ErrNoRows {
		return 0, err
	}

	// Create
	res, err := m.db.ExecContext(ctx, "INSERT INTO hierarchy_nodes (parent_id, type, name, path) VALUES (?, ?, ?, ?)",
		parentID, nodeType, name, path)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AddRiskTag inserts a risk tag for a node.
func (m *Manager) AddRiskTag(ctx context.Context, tag types.RiskTag) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO pii_tags (node_id, rule_id, risk_score, count)
		VALUES (?, ?, ?, ?)
	`, tag.NodeID, tag.RuleID, tag.RiskScore, tag.Count)
	return err
}
