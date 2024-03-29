package util

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/tychoish/fun/adt"
)

var (
	hostNameCache *adt.Once[string]
	homeDirCache  *adt.Once[string]
)

func init() {
	hostNameCache = &adt.Once[string]{}
	homeDirCache = &adt.Once[string]{}

	hostNameCache.Set(func() string {
		name, err := os.Hostname()
		if err != nil {
			return "UNKNOWN_HOSTNAME"
		}
		return name
	})

	homeDirCache.Set(func() string {
		if runtime.GOOS == "windows" {
			if dir := os.Getenv("HOME"); dir != "" {
				return dir
			} else if dir := os.Getenv("USERPROFILE"); dir != "" {
				return dir
			}

			drive := os.Getenv("HOMEDRIVE")
			path := os.Getenv("HOMEPATH")
			if drive != "" && path != "" {
				return fmt.Sprint(drive, path)
			}
			return ""
		}
		var envVar string
		if runtime.GOOS == "plan9" {
			envVar = "home"
		} else {
			envVar = "HOME"
		}

		if dir := os.Getenv(envVar); dir != "" {
			return dir
		}

		cmd := exec.Command("sh", "-c", "cd && pwd")
		out, err := cmd.Output()
		out = bytes.TrimSpace(out)
		if err != nil || len(out) == 0 {
			return "UNKNOWN_HOMEDIR"
		}

		return string(out)
	})
}

func GetHostname() string { return hostNameCache.Resolve() }
func GetHomedir() string  { return homeDirCache.Resolve() }

func TryExpandHomedir(in string) string {
	if len(in) == 0 {
		return ""
	}

	if in[0] != '~' {
		return in
	}

	if len(in) > 1 && in[1] != '/' && in[1] != '\\' {
		// these are "~foo" or "~\" values which are ambiguous
		return in
	}

	return filepath.Join(GetHomedir(), in[1:])
}
