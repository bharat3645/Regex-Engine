package connectors

import (
	"github.com/bharat3645/compliance-manager/checksum"
	"github.com/bharat3645/compliance-manager/database"
	"github.com/bharat3645/compliance-manager/logger"
	"github.com/bharat3645/compliance-manager/types"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// LocalFSConnector implements DataSource for local file systems
type LocalFSConnector struct {
	db     *database.Manager
	logger *logger.AppLogger
}

func NewLocalFS(db *database.Manager, logger *logger.AppLogger) *LocalFSConnector {
	return &LocalFSConnector{
		db:     db,
		logger: logger,
	}
}

func (c *LocalFSConnector) Name() string {
	return "Local Filesystem"
}

func (c *LocalFSConnector) Connect(ctx context.Context, config map[string]string) error {
	// Local connector is always connected
	return nil
}

func (c *LocalFSConnector) Walk(ctx context.Context, root string, jobs chan<- types.FileJob) error {
	defer close(jobs)

	// Initialize Root Nodes (Infra -> Machine)
	infraID, err := c.db.GetOrCreateNode(ctx, 0, "Infrastructure", types.NodeTypeInfra, "ROOT")
	if err != nil {
		c.logger.Error("Failed to create Infra node", "error", err)
	}
	hostname, _ := os.Hostname()
	machineID, err := c.db.GetOrCreateNode(ctx, infraID, hostname, types.NodeTypeServer, "ROOT/"+hostname)
	if err != nil {
		c.logger.Error("Failed to create Machine node", "error", err)
	}

	sem := make(chan struct{}, runtime.NumCPU()*2)
	var wg sync.WaitGroup

	// Cache to store Path -> NodeID for folders to avoid DB unnecessary lookups
	// Since WalkDir is deterministic (top-down), we can manage parent IDs efficiently.
	// For a massive system, this map needs LRU, but for a "Spoke Agent" on a single machine,
	// keeping active folder paths in memory is acceptable or we rely on DB definition.
	// Problem: WalkDir is parallelized below? No, WalkDir is sequential, but I was running goroutines per drive.
	
	walkDir := func(startPath string) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()

		// Create Drive Node
		driveName := filepath.VolumeName(startPath)
		if driveName == "" {
			driveName = "Root"
		}
		driveID, err := c.db.GetOrCreateNode(ctx, machineID, driveName, types.NodeTypeDrive, startPath)
		if err != nil {
			c.logger.Error("Failed to create drive node", "path", startPath, "error", err)
			return
		}

		// Helper to resolve parent ID. 
		// We use a map for the current walk.
		// NOTE: filepath.WalkDir is sequential.
		// path -> nodeID
		folderCache := make(map[string]int64)
		folderCache[startPath] = driveID

		filepath.WalkDir(startPath, func(path string, d fs.DirEntry, err error) error {
			select {
			case <-ctx.Done():
				return filepath.SkipAll
			default:
			}

			if err != nil || d.Type()&fs.ModeSymlink != 0 {
				return nil
			}

			// Normalize path for consistent map lookup
			normalizedPath := filepath.Clean(path)
			parentPath := filepath.Dir(normalizedPath)
			
			parentID, ok := folderCache[parentPath]
			if !ok {
				// Fallback: Try parent's parent or Drive (Edge case for root files)
				if strings.EqualFold(parentPath, startPath) {
					parentID = driveID
				} else {
					// We might have missed it if WalkDir order is weird or it's a root.
					parentID = driveID
				}
			}

			if d.IsDir() {
				// Create Folder Node
				nodeID, err := c.db.GetOrCreateNode(ctx, parentID, d.Name(), types.NodeTypeFolder, normalizedPath)
				if err != nil {
					return nil // Skip children if we fail to create folder
				}
				folderCache[normalizedPath] = nodeID
				return nil
			}

			// It is a file: Check Incremental Scan Logic
			info, err := d.Info()
			if err != nil {
				return nil
			}
			modifiedTime := info.ModTime().Unix()

			cachedEntry, err := c.db.GetFileCacheEntry(ctx, path)
			if err != nil {
				return nil
			}

			if cachedEntry != nil && cachedEntry.ModifiedTime == modifiedTime {
				return nil
			}

			currentChecksum, err := checksum.CalculateFileChecksum(path)
			if err != nil {
				return nil
			}

			if cachedEntry != nil && cachedEntry.Checksum == currentChecksum {
				cachedEntry.ModifiedTime = modifiedTime
				c.db.UpdateFileCacheEntry(ctx, *cachedEntry)
				return nil
			}

			// Create File Node attached to correct Parent Folder
			fileNodeID, err := c.db.GetOrCreateNode(ctx, parentID, d.Name(), types.NodeTypeFile, path)
			if err != nil {
				return nil
			}

			select {
			case jobs <- types.FileJob{
				NodeID:    fileNodeID,
				Path:      path,
				Extension: strings.ToLower(filepath.Ext(path)),
			}:
			case <-ctx.Done():
				return filepath.SkipAll
			}

			return nil
		})
	}

	if root != "" {
		c.logger.Info("Starting targeted discovery", "root", root)
		wg.Add(1)
		go walkDir(root)
	} else {
		c.logger.Info("Starting full system discovery...")
		if runtime.GOOS == "windows" {
			for _, drive := range getWindowsDrives() {
				wg.Add(1)
				go walkDir(drive)
			}
		} else {
			wg.Add(1)
			go walkDir("/")
		}
	}

	wg.Wait()
	return nil
}

// Helper to get drives
func getWindowsDrives() []string {
	var drives []string
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(drive) + ":\\"
		if _, err := os.Stat(path); err == nil {
			drives = append(drives, path)
		}
	}
	return drives
}
