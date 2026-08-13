package connectors

import (
	"github.com/bharat3645/compliance-manager/types"
	"context"
)

// DataSource represents a generic source of data (FileSystem, Cloud Storage, or Database).
// It abstracts the traversal logic so the Engine doesn't care if it's scanning C:\ or S3://bucket.
type DataSource interface {
	// Name returns the display name of the connector (e.g., "Local Filesystem", "AWS S3").
	Name() string

	// Connect verifies accessibility to the resource.
	Connect(ctx context.Context, config map[string]string) error

	// Walk traverses the data source and streams found items to the output channel.
	// It is responsible for creating hierarchy nodes in the DB as it encounters folders/containers.
	Walk(ctx context.Context, root string, output chan<- types.FileJob) error
}
