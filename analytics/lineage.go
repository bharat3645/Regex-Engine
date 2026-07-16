package analytics

import (
	"Regex/database"
	"context"
	"fmt"
)

// LineageEngine infers relationships between files
type LineageEngine struct {
	DB *database.Manager
}

func NewLineageEngine(db *database.Manager) *LineageEngine {
	return &LineageEngine{DB: db}
}

// BuildRelations identifies duplicates and creates 'CopyOf' edges in the graph
func (e *LineageEngine) BuildRelations(ctx context.Context) error {
	// 1. Find Exact Duplicates using Checksum (MD5/SHA256 from file_cache)
	// We need to join file_cache with hierarchy_nodes to get Node IDs
	query := `
		SELECT n1.id, n2.id
		FROM file_cache f1
		JOIN file_cache f2 ON f1.checksum = f2.checksum AND f1.path != f2.path
		JOIN hierarchy_nodes n1 ON f1.path = n1.path
		JOIN hierarchy_nodes n2 ON f2.path = n2.path
		WHERE n1.id < n2.id -- Avoid double counting (A-B and B-A)
	`

	rows, err := e.DB.DB().QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("lineage query failed: %w", err)
	}
	defer rows.Close()

	tx, err := e.DB.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO node_relations (source_node_id, target_node_id, type, weight)
		VALUES (?, ?, 'CopyOf', 1.0)
	`)
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	count := 0
	for rows.Next() {
		var id1, id2 int64
		if err := rows.Scan(&id1, &id2); err != nil {
			continue
		}

		if _, err := insertStmt.ExecContext(ctx, id1, id2); err != nil {
			continue
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Printf("Lineage Analysis: Found %d duplicate relations.\n", count)
	return nil
}
