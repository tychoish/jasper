package testutil

import (
	"path/filepath"
	"runtime"
)

func BuildDirectory() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(file)), "build")
}
