package connectors

import (
	"github.com/bharat3645/compliance-manager/database"
	"github.com/bharat3645/compliance-manager/logger"
	"fmt"
)

// Factory creates connectors based on configuration.
type Factory struct {
	db     *database.Manager
	logger *logger.AppLogger
}

func NewFactory(db *database.Manager, logger *logger.AppLogger) *Factory {
	return &Factory{
		db:     db,
		logger: logger,
	}
}

// GetConnector returns the appropriate DataSource.
func (f *Factory) GetConnector(sourceType string) (DataSource, error) {
	switch sourceType {
	case "local_fs", "filesystem", "":
		return NewLocalFS(f.db, f.logger), nil
	case "s3":
		return nil, fmt.Errorf("s3 connector not implemented yet") // Placeholder
	default:
		return nil, fmt.Errorf("unknown data source type: %s", sourceType)
	}
}
