package jamboy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tychoish/amboy"
	"github.com/tychoish/amboy/dependency"
	"github.com/tychoish/amboy/job"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper/x/remote/options"
)

const downloadJobName = "jasper-download-job"

// downloadFileJob is an amboy.Job implementation that supports
// downloading a a file to the local file system.
type downloadFileJob struct {
	URL       string `bson:"url" json:"url" yaml:"url"`
	Directory string `bson:"dir" json:"dir" yaml:"dir"`
	FileName  string `bson:"file" json:"file" yaml:"file"`
	*job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func newDownloadJob() *downloadFileJob {
	return &downloadFileJob{
		Base: &job.Base{
			JobType: amboy.JobType{
				Name:    downloadJobName,
				Version: 0,
			},
		},
	}
}

// NewDownloadJob constructs a new amboy-compatible Job that can
// download and extract (if tarball) a file to the local
// filesystem. The job has a dependency on the downloaded file's path, and
// will only execute if that file does not exist, unless the force
// flag is passed.
func NewDownloadJob(url, path string, force bool) (amboy.Job, error) {
	j := newDownloadJob()
	if err := j.setURL(url); err != nil {
		return nil, fmt.Errorf("problem constructing Job object (url): %w", err)
	}

	if err := j.setDirectory(path); err != nil {
		return nil, fmt.Errorf("problem constructing Job object (directory): %w", err)
	}

	fn := j.getFileName()
	j.SetID(fmt.Sprintf("%s-%s-%d",
		j.Type().Name,
		strings.Replace(fn, string(filepath.Separator), "-", -1),
		job.GetNumber()))

	if force {
		_ = os.Remove(fn)
		_ = os.RemoveAll(fn[:len(fn)-4])
		j.SetDependency(dependency.NewAlways())
	} else {
		j.SetDependency(dependency.NewCreatesFile(fn))
	}

	return j, nil
}

// Run implements the main action of the Job. This implementation
// checks the job directly and returns early if the downloaded file
// exists. This behavior may be redundant in the case that the queue
// skips jobs with "passed" jobs.
func (j *downloadFileJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	fn := j.getFileName()
	defer attemptTimestampUpdate(fn)

	// in theory the queue should do this next check, but most
	// unordered implementations do not
	if state := j.Dependency().State(); state == dependency.Passed {
		grip.Debug(message.Fields{
			"file":    fn,
			"message": "file is already downloaded",
			"op":      "none",
		})
		return
	}

	opts := options.Download{
		URL:  j.URL,
		Path: fn,
		ArchiveOpts: options.Archive{
			Format:        options.ArchiveAuto,
			ShouldExtract: true,
			TargetPath:    getTargetDirectory(fn),
		},
	}

	if err := opts.Download(ctx); err != nil {
		j.handleError(fmt.Errorf("problem downloading file %s: %w", fn, err))
		return
	}

	grip.Debug(message.Fields{
		"op":   "downloaded file complete",
		"file": fn,
	})
}

//
// Internal
//

func getTargetDirectory(fn string) string {
	baseName := filepath.Base(fn)
	return filepath.Join(filepath.Dir(fn), baseName[:len(baseName)-len(filepath.Ext(baseName))])
}

func attemptTimestampUpdate(fn string) {
	// update the timestamps so we playwell with the cache. These
	// operations are logged but don't impact the tasks error
	// state if they fail.
	now := time.Now()
	if err := os.Chtimes(fn, now, now); err != nil {
		grip.Debug(err)
	}

	// hopefully directory names in archives are the same are the
	// same as the filenames. Unwinding this assumption will
	// probably require a different archiver tool.
	dirname := fn[0 : len(fn)-len(filepath.Ext(fn))]
	if err := os.Chtimes(dirname, now, now); err != nil {
		grip.Debug(err)
	}
}

func (j *downloadFileJob) handleError(err error) {
	j.AddError(err)

	grip.Error(message.WrapError(err, message.Fields{
		"message": "problem downloading file",
		"name":    j.FileName,
		"op":      "cleaning up artifacts",
	}))
	grip.Warning(os.RemoveAll(j.getFileName())) // cleanup
}

func (j *downloadFileJob) getFileName() string {
	return filepath.Join(j.Directory, j.FileName)
}

func (j *downloadFileJob) setDirectory(path string) error {
	if stat, err := os.Stat(path); !os.IsNotExist(err) && !stat.IsDir() {
		// if the path exists and isn't a directory, then we
		// won't be able to download into it:
		return fmt.Errorf("%s is not a directory, cannot download files into it", path)
	}

	j.Directory = path
	return nil
}

func (j *downloadFileJob) setURL(url string) error {
	if !strings.HasPrefix(url, "http") {
		return fmt.Errorf("%s is not a valid url", url)
	}

	if strings.HasSuffix(url, "/") {
		return fmt.Errorf("%s does not contain a valid filename component", url)
	}

	j.URL = url
	j.FileName = filepath.Base(url)

	if strings.HasSuffix(url, ".tar.gz") {
		j.FileName = filepath.Ext(filepath.Ext(j.FileName)) + ".tgz"
	}

	return nil
}
