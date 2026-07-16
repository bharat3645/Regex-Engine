// File: checksum/checksum.go
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// CalculateFileChecksum computes the SHA256 hash of a file by streaming it.
// This is memory-efficient and can handle very large files.
func CalculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
