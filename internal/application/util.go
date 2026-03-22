package application

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// safeFilename returns a filesystem-safe base name for the given storage path.
// If the name exceeds 200 characters it is shortened and a hash suffix is added
// to avoid collisions.
func safeFilename(path string) string {
	// Strip any query string (e.g. from signed URLs)
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	baseName := filepath.Base(path)

	if len(baseName) > 200 {
		hash := sha256.Sum256([]byte(path))
		extension := filepath.Ext(baseName)

		shortName := baseName
		if len(baseName) > 20 {
			shortName = baseName[:20]
		}

		shortName = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`<>:"/\|?*`, r) {
				return '_'
			}
			return r
		}, shortName)

		baseName = fmt.Sprintf("%s-%s%s", shortName, hex.EncodeToString(hash[:8]), extension)
	}

	return baseName
}
