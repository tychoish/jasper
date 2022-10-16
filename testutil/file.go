package testutil

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mholt/archiver"

	"github.com/tychoish/emt"
)

// AddFileToDirectory adds an archive file given by fileName with the given
// fileContents to the directory.
func AddFileToDirectory(dir string, fileName string, fileContents string) error {
	if format, _ := archiver.ByExtension(fileName); format != nil {
		builder, ok := format.(archiver.Archiver)
		if !ok {
			return errors.New("unsupported archive format")
		}
		tmpFile, err := ioutil.TempFile(dir, "tmp.txt")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpFile.Name())
		if _, err := tmpFile.Write([]byte(fileContents)); err != nil {
			catcher := emt.NewBasicCatcher()
			catcher.Add(err)
			catcher.Add(tmpFile.Close())
			return catcher.Resolve()
		}
		if err := tmpFile.Close(); err != nil {
			return err
		}

		if err := builder.Archive([]string{tmpFile.Name()}, filepath.Join(dir, fileName)); err != nil {
			return err
		}
		return nil
	}

	file, err := os.Create(filepath.Join(dir, fileName))
	if err != nil {
		return err
	}
	if _, err := file.Write([]byte(fileContents)); err != nil {
		catcher := emt.NewBasicCatcher()
		catcher.Add(err)
		catcher.Add(file.Close())
		return catcher.Resolve()
	}
	return file.Close()
}

func BuildDirectory() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(file)), "build")
}
