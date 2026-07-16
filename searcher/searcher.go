package searcher

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// A list of all file extensions that the extractor supports.
var supportedExtensions = map[string]bool{
	".txt":  true,
	".xml":  true,
	".docx": true,
	// ".pdf": true,
	// ".xlsx": true,
	// ".pptx": true,
}

// FindFiles starts a file scan. If a root directory is provided, it scans only that
// directory. If root is an empty string, it performs a full system scan.
func FindFiles(root string, jobs chan<- string) {
	defer close(jobs)

	if root != "" {
		slog.Info("Starting targeted scan for supported files", "root", root)
		walk(root, jobs)
	} else {
		slog.Info("Starting full system scan for supported files...")
		if runtime.GOOS == "windows" {
			drives := getWindowsDrives()
			slog.Info("Detected Windows OS", "drives", drives)
			for _, drive := range drives {
				slog.Info("Starting walk on drive", "drive", drive)
				walk(drive, jobs)
			}
		} else {
			slog.Info("Detected Unix-like OS, starting walk from '/'")
			walk("/", jobs)
		}
	}
	slog.Info("File discovery completed.")
}

func walk(root string, jobs chan<- string) {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("Skipping path due to access error", "path", path, "error", err)
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if !d.IsDir() && supportedExtensions[extension] {
			jobs <- path
		}
		return nil
	})
	if err != nil {
		slog.Error("File system walk failed for root", "root", root, "error", err)
	}
}

func getWindowsDrives() []string {
	var drives []string
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := fmt.Sprintf("%c:\\", drive)
		if _, err := os.Stat(path); err == nil {
			drives = append(drives, path)
		}
	}
	return drives
}
