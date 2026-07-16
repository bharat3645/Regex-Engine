package analytics

import (
	"Regex/database"
	"context"
	"fmt"
	"strings"
)

// ClusterCallback defines how to notify when a cluster is found
type ClusterCallback func(clusterID string, filePaths []string)

// ClusteringEngine analyzes PII data to find groups
type ClusteringEngine struct {
	DB *database.Manager
}

func NewClusteringEngine(db *database.Manager) *ClusteringEngine {
	return &ClusteringEngine{DB: db}
}

// ClusterByPIIType groups files that share a high volume of specific PII types
// This is a simple "Rule-Based" cluster: "PCI Data Cluster", "PHI Data Cluster"
func (c *ClusteringEngine) ClusterByPIIType(ctx context.Context) (map[string][]string, error) {
	// Query to find files with validated PII of specific types
	// Group by file_path and pii_type_refined
	query := `
		SELECT n.path, t.pii_type_refined, COUNT(*) as count 
		FROM pii_tags t
		JOIN hierarchy_nodes n ON t.node_id = n.id
		WHERE t.validation_status = 'validated' 
		GROUP BY n.path, t.pii_type_refined
		HAVING count >= 2
	`

	rows, err := c.DB.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("clustering query failed: %w", err)
	}
	defer rows.Close()

	clusters := make(map[string][]string)

	for rows.Next() {
		var path, piiType string
		var count int
		if err := rows.Scan(&path, &piiType, &count); err != nil {
			continue
		}

		clusterName := fmt.Sprintf("Cluster: High Density %s", strings.ToUpper(piiType))
		clusters[clusterName] = append(clusters[clusterName], path)
	}

	return clusters, nil
}

// VectorSimilarityCluster would go here (requires vector math lib)
