package application

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points TMPDIR at a directory private to this test binary, so
// gallery.WorkDir() is not shared with other packages' tests. `go test ./...`
// runs packages in parallel, and the temp-file leak checks in this package
// count WorkDir entries, which another package writing there would skew.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "application-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("TMPDIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
