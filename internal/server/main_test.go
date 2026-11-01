package server

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain removes the directory hhBinary built hedgehug into, which no test
// owns (the binary is built once for all of them).
func TestMain(m *testing.M) {
	code := m.Run()
	if hhBin != "" {
		_ = os.RemoveAll(filepath.Dir(hhBin))
	}
	os.Exit(code)
}
