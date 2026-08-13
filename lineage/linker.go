package lineage

import (
	"github.com/bharat3645/compliance-manager/database"
	"github.com/bharat3645/compliance-manager/logger"
	"context"
	"database/sql"
	"time"
)

// Linker analyzes the metadata to find relationships between nodes.
type Linker struct {
	db     *database.Manager
	logger *logger.AppLogger
}

func NewLinker(db *database.Manager, logger *logger.AppLogger) *Linker {
	return &Linker{
		db:     db,
		logger: logger,
	}
}

// FindExactDuplicates links files with the same checksum (CopyOf).
func (l *Linker) FindExactDuplicates(ctx context.Context) error {
	l.logger.Info("Lineage: Starting analysis for exact duplicates (CopyOf)...")
	
	// Self-join on file_cache to find same checksums with different paths
	// In a real Spoke-Hub, this would run on the Hub across all agents.
	// For Local Spoke, it finds local duplicates.
	
	// We need to join file_cache (contains checksum) with hierarchy_nodes (contains ID).
	// Simplification: We added hierarchy, but kept file_cache separated for now.
	// Ideal state: file_cache should reference node_id.
	// WORKAROUND: We will join on PATH.
	
	query := `
		SELECT n1.id AS source, n2.id AS target
		FROM file_cache f1
		JOIN file_cache f2 ON f1.checksum = f2.checksum AND f1.path != f2.path
		JOIN hierarchy_nodes n1 ON n1.path = f1.path
		JOIN hierarchy_nodes n2 ON n2.path = f2.path
		WHERE n1.id < n2.id -- Avoid double counting (A->B and B->A)
	`
	
	rows, err := l.db.DB().QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var u, v int64
		if err := rows.Scan(&u, &v); err != nil {
			continue
		}
		
		// Insert Edge
		err := l.AddEdge(ctx, u, v, "CopyOf")
		if err != nil {
			l.logger.Warn("Failed to add edge", "u", u, "v", v, "error", err)
		} else {
			count++
		}
	}
	
	l.logger.Info("Lineage: Finished duplicate analysis", "new_edges", count)
	return nil
}

func (l *Linker) AddEdge(ctx context.Context, u, v int64, relType string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check if exists
	var id int64
	err := l.db.DB().QueryRowContext(ctx, "SELECT id FROM node_relations WHERE source_node_id = ? AND target_node_id = ? AND type = ?", u, v, relType).Scan(&id)
	if err == nil {
		return nil // Already exists
	} else if err != sql.ErrNoRows {
		return err
	}

	_, err = l.db.DB().ExecContext(ctx, `
		INSERT INTO node_relations (source_node_id, target_node_id, type)
		VALUES (?, ?, ?)
	`, u, v, relType)
	return err
}
