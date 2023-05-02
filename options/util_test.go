package options

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestMakeEnclosingDirectories(t *testing.T) {
	path := "foo"
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
	check.NotError(t, makeEnclosingDirectories(path))
	defer os.RemoveAll(path)

	_, path, _, ok := runtime.Caller(0)
	assert.True(t, ok)
	info, err := os.Stat(path)
	assert.True(t, !os.IsNotExist(err))
	assert.True(t, !info.IsDir())
	check.Error(t, makeEnclosingDirectories(path))
}

func TestWriteFile(t *testing.T) {
	for testName, testCase := range map[string]struct {
		content    string
		path       string
		shouldPass bool
	}{
		"FailsForInsufficientMkdirPermissions": {
			content:    "foo",
			path:       "/bar",
			shouldPass: false,
		},
		"FailsForInsufficientFileWritePermissions": {
			content:    "foo",
			path:       "/etc/hosts",
			shouldPass: false,
		},
		"FailsForInsufficientFileOpenPermissions": {
			content:    "foo",
			path:       "/etc/whatever",
			shouldPass: false,
		},
		"WriteToFileSucceeds": {
			content:    "foo",
			path:       "/dev/null",
			shouldPass: true,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("cannot test download permissions as root")
			} else if runtime.GOOS == "windows" {
				t.Skip("cannot run file write tests on windows")
			}
			err := writeFile(bytes.NewBufferString(testCase.content), testCase.path)
			if testCase.shouldPass {
				check.NotError(t, err)
			} else {
				check.Error(t, err)
			}
		})
	}
}

func TestWriteFileOptions(t *testing.T) {
	for opName, opCases := range map[string]func(t *testing.T){
		"Validate": func(t *testing.T) {
			for testName, testCase := range map[string]func(t *testing.T){
				"FailsForZeroValue": func(t *testing.T) {
					opts := WriteFile{}
					check.Error(t, opts.Validate())
				},
				"OnlyDefaultsPermForZeroValue": func(t *testing.T) {
					opts := WriteFile{Path: "/foo", Perm: 0777}
					check.NotError(t, opts.Validate())
					check.Equal(t, 0777, opts.Perm)
				},
				"PassesAndDefaults": func(t *testing.T) {
					opts := WriteFile{Path: "/foo"}
					check.NotError(t, opts.Validate())
					check.NotEqual(t, os.FileMode(0000), opts.Perm)
				},
				"PassesWithContent": func(t *testing.T) {
					opts := WriteFile{
						Path:    "/foo",
						Content: []byte("foo"),
					}
					check.NotError(t, opts.Validate())
				},
				"PassesWithReader": func(t *testing.T) {
					opts := WriteFile{
						Path:   "/foo",
						Reader: bytes.NewBufferString("foo"),
					}
					check.NotError(t, opts.Validate())
				},
				"FailsWithMultipleContentSources": func(t *testing.T) {
					opts := WriteFile{
						Path:    "/foo",
						Content: []byte("foo"),
						Reader:  bytes.NewBufferString("bar"),
					}
					check.Error(t, opts.Validate())
				},
			} {
				t.Run(testName, func(t *testing.T) {
					testCase(t)
				})
			}
		},
		"ContentReader": func(t *testing.T) {
			for testName, testCase := range map[string]func(t *testing.T, opts WriteFile){
				"RequiresOneContentSource": func(t *testing.T, opts WriteFile) {
					opts.Content = []byte("foo")
					opts.Reader = bytes.NewBufferString("bar")
					_, err := opts.ContentReader()
					check.Error(t, err)
				},
				"PreservesReaderIfSet": func(t *testing.T, opts WriteFile) {
					expected := []byte("foo")
					opts.Reader = bytes.NewBuffer(expected)
					reader, err := opts.ContentReader()
					assert.NotError(t, err)
					check.True(t, opts.Reader == reader)

					content, err := io.ReadAll(reader)
					assert.NotError(t, err)
					check.EqualItems(t, expected, content)
				},
				"SetsReaderIfContentSet": func(t *testing.T, opts WriteFile) {
					expected := []byte("foo")
					opts.Content = expected
					reader, err := opts.ContentReader()
					assert.NotError(t, err)
					check.True(t, reader == opts.Reader)
					check.Equal(t, len(opts.Content), 0)

					content, err := io.ReadAll(reader)
					assert.NotError(t, err)
					check.EqualItems(t, expected, content)
				},
			} {
				t.Run(testName, func(t *testing.T) {
					opts := WriteFile{Path: "/path"}
					testCase(t, opts)
				})
			}
		},
		"WriteBufferedContent": func(t *testing.T) {
			for testName, testCase := range map[string]func(t *testing.T, opts WriteFile){
				"DoesNotErrorWithoutContentSource": func(t *testing.T, opts WriteFile) {
					didWrite := false
					check.NotError(t, opts.WriteBufferedContent(func(WriteFile) error {
						didWrite = true
						return nil
					}))
					check.True(t, didWrite)
				},
				"FailsForMultipleContentSources": func(t *testing.T, opts WriteFile) {
					opts.Content = []byte("foo")
					opts.Reader = bytes.NewBufferString("bar")
					check.Error(t, opts.WriteBufferedContent(func(WriteFile) error { return nil }))
				},
				"ReadsFromContent": func(t *testing.T, opts WriteFile) {
					expected := []byte("foo")
					opts.Content = expected
					content := []byte{}
					assert.NotError(t, opts.WriteBufferedContent(func(opts WriteFile) error {
						content = append(content, opts.Content...)
						return nil
					}))
					check.EqualItems(t, expected, content)
				},
				"ReadsFromReader": func(t *testing.T, opts WriteFile) {
					expected := []byte("foo")
					opts.Reader = bytes.NewBuffer(expected)
					content := []byte{}
					assert.NotError(t, opts.WriteBufferedContent(func(opts WriteFile) error {
						content = append(content, opts.Content...)
						return nil
					}))
					check.EqualItems(t, expected, content)
				},
			} {
				t.Run(testName, func(t *testing.T) {
					opts := WriteFile{Path: "/path"}
					testCase(t, opts)
				})
			}
		},
		"DoWrite": func(t *testing.T) {
			content := []byte("foo")
			for testName, testCase := range map[string]func(t *testing.T, opts WriteFile){
				"AllowsEmptyWriteToCreateFile": func(t *testing.T, opts WriteFile) {
					assert.NotError(t, opts.DoWrite())

					stat, err := os.Stat(opts.Path)
					assert.NotError(t, err)
					check.Zero(t, stat.Size())
				},
				"WritesWithReader": func(t *testing.T, opts WriteFile) {
					opts.Reader = bytes.NewBuffer(content)

					assert.NotError(t, opts.DoWrite())

					fileContent, err := os.ReadFile(opts.Path)
					assert.NotError(t, err)
					check.EqualItems(t, content, fileContent)
				},
				"WritesWithContent": func(t *testing.T, opts WriteFile) {
					opts.Content = content

					assert.NotError(t, opts.DoWrite())

					fileContent, err := os.ReadFile(opts.Path)
					assert.NotError(t, err)
					check.EqualItems(t, content, fileContent)
				},
				"AppendsToFile": func(t *testing.T, opts WriteFile) {
					f, err := os.OpenFile(opts.Path, os.O_WRONLY|os.O_CREATE, 0666)
					initialContent := []byte("bar")
					assert.NotError(t, err)
					_, err = f.Write(initialContent)
					assert.NotError(t, err)
					assert.NotError(t, f.Close())

					opts.Append = true
					opts.Content = content

					assert.NotError(t, opts.DoWrite())

					fileContent, err := os.ReadFile(opts.Path)
					assert.NotError(t, err)
					check.EqualItems(t, initialContent, fileContent[:len(initialContent)])
					check.EqualItems(t, content, fileContent[len(fileContent)-len(content):])
				},
				"TruncatesExistingFile": func(t *testing.T, opts WriteFile) {
					f, err := os.OpenFile(opts.Path, os.O_WRONLY|os.O_CREATE, 0666)
					initialContent := []byte("bar")
					assert.NotError(t, err)
					_, err = f.Write(initialContent)
					assert.NotError(t, err)
					assert.NotError(t, f.Close())

					opts.Content = content

					assert.NotError(t, opts.DoWrite())

					fileContent, err := os.ReadFile(opts.Path)
					assert.NotError(t, err)
					check.EqualItems(t, content, fileContent)
				},
			} {
				t.Run(testName, func(t *testing.T) {
					// TODO: we can't use testutil.BuildDirectory() because it
					// will cause a cycle.
					cwd, err := os.Getwd()
					assert.NotError(t, err)
					opts := WriteFile{Path: filepath.Join(filepath.Dir(cwd), filepath.Base(t.Name()))}
					defer func() {
						check.NotError(t, os.RemoveAll(opts.Path))
					}()
					testCase(t, opts)
				})
			}
		},
		"SetPerm": func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("permission tests are not relevant to Windows")
			}
			for testName, testCase := range map[string]func(t *testing.T, opts WriteFile){
				"SetsPermissions": func(t *testing.T, opts WriteFile) {
					f, err := os.OpenFile(opts.Path, os.O_RDWR|os.O_CREATE, 0666)
					assert.NotError(t, err)
					assert.NotError(t, f.Close())

					opts.Perm = 0400
					assert.NotError(t, opts.SetPerm())

					stat, err := os.Stat(opts.Path)
					assert.NotError(t, err)
					check.Equal(t, opts.Perm, stat.Mode())
				},
				"FailsWithoutFile": func(t *testing.T, opts WriteFile) {
					opts.Perm = 0400
					check.Error(t, opts.SetPerm())
				},
			} {
				t.Run(testName, func(t *testing.T) {
					// TODO: we can't use testutil.BuildDirectory() because it
					// will cause a cycle.
					cwd, err := os.Getwd()
					assert.NotError(t, err)
					opts := WriteFile{Path: filepath.Join(filepath.Dir(cwd), filepath.Base(t.Name()))}
					defer func() {
						check.NotError(t, os.RemoveAll(opts.Path))
					}()
					testCase(t, opts)
				})
			}
		},
	} {
		t.Run(opName, func(t *testing.T) {
			opCases(t)
		})
	}
}
