package jamboy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mholt/archiver"
	"github.com/tychoish/amboy/queue"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/x/remote/options"
)

func TestCreateValidDownloadJobs(t *testing.T) {
	dir, err := os.MkdirTemp(testutil.BuildDirectory(), "out")
	assert.NotError(t, err)
	defer os.RemoveAll(dir)

	urls := make(chan string)
	go func() {
		urls <- "https://example.com"
		close(urls)
	}()

	catcher := &erc.Collector{}
	jobs := createDownloadJobs(dir, urls, catcher)

	count := 0
	for job := range jobs {
		count++
		assert.Equal(t, 1, count)
		assert.True(t, job != nil)
	}

	check.NotError(t, catcher.Resolve())
}

func TestCreateDownloadJobsWithInvalidPath(t *testing.T) {
	_, dir, _, ok := runtime.Caller(0)
	assert.True(t, ok)
	urls := make(chan string)
	testURL := "https://example.com"

	catcher := &erc.Collector{}
	go func() {
		urls <- testURL
		close(urls)
	}()
	jobs := createDownloadJobs(dir, urls, catcher)

	for range jobs {
		t.Error("should not create job for bad url")
	}
	err := catcher.Resolve()
	assert.Error(t, err)
	check.Substring(t, err.Error(), "problem creating download job for "+testURL)
}

func TestProcessDownloadJobs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testutil.LongTestTimeout)
	defer cancel()

	downloadDir, err := os.MkdirTemp(testutil.BuildDirectory(), "download_test")
	assert.NotError(t, err)
	defer os.RemoveAll(downloadDir)

	fileServerDir, err := os.MkdirTemp(testutil.BuildDirectory(), "download_test_server")
	assert.NotError(t, err)
	defer os.RemoveAll(fileServerDir)

	fileName := "foo.zip"
	fileContents := "foo"
	assert.NotError(t, AddFileToDirectory(fileServerDir, fileName, fileContents))
	port := testutil.GetPortNumber()
	fileServerAddr := fmt.Sprintf("localhost:%d", port)
	fileServer := &http.Server{Addr: fileServerAddr, Handler: http.FileServer(http.Dir(fileServerDir))}
	defer func() {
		check.NotError(t, fileServer.Close())
	}()
	listener, err := net.Listen("tcp", fileServerAddr)
	assert.NotError(t, err)
	go func() {
		grip.Info(fileServer.Serve(listener))
	}()

	baseURL := fmt.Sprintf("http://%s", fileServerAddr)
	assert.NotError(t, testutil.WaitForRESTService(ctx, baseURL))

	job, err := NewDownloadJob(fmt.Sprintf("%s/%s", baseURL, fileName), downloadDir, true)
	assert.NotError(t, err)

	q := queue.NewLocalLimitedSize(&queue.FixedSizeQueueOptions{
		Workers:  2,
		Capacity: 1048,
		Logger:   grip.Clone(),
	})
	assert.NotError(t, q.Start(ctx))
	assert.NotError(t, q.Put(ctx, job))

	checkFileNonempty := func(fileName string) error {
		info, err := os.Stat(fileName)
		if err != nil {
			return err
		}
		if info.Size() == 0 {
			return errors.New("expected file to be non-empty")
		}
		return nil
	}
	check.NotError(t, processDownloadJobs(ctx, checkFileNonempty)(q))
}

func TestDoExtract(t *testing.T) {
	for testName, testCase := range map[string]struct {
		archiveMaker  archiver.Archiver
		expectSuccess bool
		invalidCreate bool
		fileExtension string
		format        options.ArchiveFormat
	}{
		"Auto": {
			archiveMaker:  archiver.NewTarGz(),
			expectSuccess: true,
			fileExtension: ".tar.gz",
			format:        options.ArchiveAuto,
		},
		"TarGz": {
			archiveMaker:  archiver.NewTarGz(),
			expectSuccess: true,
			fileExtension: ".tar.gz",
			format:        options.ArchiveTarGz,
		},
		"Zip": {
			archiveMaker:  archiver.NewZip(),
			expectSuccess: true,
			fileExtension: ".zip",
			format:        options.ArchiveZip,
		},
		"InvalidArchiveFormat": {
			archiveMaker:  archiver.NewTarGz(),
			expectSuccess: false,
			invalidCreate: true,
			fileExtension: ".foo",
			format:        options.ArchiveFormat("foo"),
		},
		"MismatchedArchiveFileAndFormat": {
			archiveMaker:  archiver.NewTarGz(),
			expectSuccess: false,
			fileExtension: ".tar.gz",
			format:        options.ArchiveZip,
		},
	} {
		t.Run(testName, func(t *testing.T) {
			tempDir, err := os.MkdirTemp(testutil.BuildDirectory(), "")
			assert.NotError(t, err)
			defer os.RemoveAll(tempDir)

			file, err := os.Create(filepath.Join(tempDir, "out.txt"))
			assert.NotError(t, err)
			defer os.Remove(file.Name())

			archiveFile, err := os.Create(filepath.Join(tempDir, "out"+testCase.fileExtension))
			assert.NotError(t, err)
			defer os.Remove(archiveFile.Name())
			extractDir := filepath.Join(testutil.BuildDirectory(), "out")
			assert.NotError(t, os.MkdirAll(extractDir, 0755))
			defer os.RemoveAll(extractDir)

			err = testCase.archiveMaker.Archive([]string{file.Name()}, "second-"+archiveFile.Name())
			if testCase.invalidCreate {
				assert.Error(t, err)
				return
			}
			assert.NotError(t, err)

			opts := options.Download{
				Path: "second-" + archiveFile.Name(),
				ArchiveOpts: options.Archive{
					ShouldExtract: true,
					Format:        testCase.format,
					TargetPath:    extractDir,
				},
			}
			if !testCase.expectSuccess {
				check.Error(t, opts.Extract())
				return
			}
			check.NotError(t, opts.Extract())

			fileInfo, err := os.Stat("second-" + archiveFile.Name())
			assert.NotError(t, err)
			assert.NotZero(t, fileInfo.Size())

			dirEntry, err := os.ReadDir(extractDir)
			assert.NotError(t, err)
			assert.Equal(t, 1, len(dirEntry))
		})
	}
}

func TestDoExtractUnarchivedFile(t *testing.T) {
	file, err := os.CreateTemp(testutil.BuildDirectory(), "out.txt")
	assert.NotError(t, err)
	defer os.Remove(file.Name())

	opts := options.Download{
		URL:  "https://example.com",
		Path: file.Name(),
		ArchiveOpts: options.Archive{
			ShouldExtract: true,
			Format:        options.ArchiveAuto,
			TargetPath:    "build",
		},
	}
	err = opts.Extract()
	assert.Error(t, err)
	assert.Substring(t, err.Error(), "could not detect archive format")
}

// AddFileToDirectory adds an archive file given by fileName with the given
// fileContents to the directory.
func AddFileToDirectory(dir string, fileName string, fileContents string) error {
	if format, _ := archiver.ByExtension(fileName); format != nil {
		builder, ok := format.(archiver.Archiver)
		if !ok {
			return errors.New("unsupported archive format")
		}

		tmpFile, err := os.CreateTemp(dir, "tmp.txt")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpFile.Name())
		if _, err := tmpFile.Write([]byte(fileContents)); err != nil {
			catcher := &erc.Collector{}
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
		catcher := &erc.Collector{}
		catcher.Add(err)
		catcher.Add(file.Close())
		return catcher.Resolve()
	}
	return file.Close()
}
