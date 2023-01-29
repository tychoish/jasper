package options

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tychoish/fun/erc"
)

func makeEnclosingDirectories(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(path, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
	} else if !info.IsDir() {
		return fmt.Errorf("'%s' already exists and is not a directory", path)
	}
	return nil
}

func writeFile(reader io.Reader, path string) error {
	if err := makeEnclosingDirectories(filepath.Dir(path)); err != nil {
		return fmt.Errorf("problem making enclosing directories: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("problem creating file: %w", err)
	}

	catcher := &erc.Collector{}
	if _, err := io.Copy(file, reader); err != nil {
		catcher.Add(fmt.Errorf("problem writing file: %w", err))
	}

	if err := file.Close(); err != nil {
		catcher.Add(fmt.Errorf("problem closing %q: %w", path, err))
	}

	return catcher.Resolve()
}
