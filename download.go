package jasper

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/queue"
	"github.com/deciduosity/bond/recall"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/recovery"
	"github.com/pkg/errors"
)

func makeEnclosingDirectories(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(path, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
	} else if !info.IsDir() {
		return errors.Errorf("'%s' already exists and is not a directory", path)
	}
	return nil
}

func createDownloadJobs(path string, urls <-chan string, catcher grip.Catcher) <-chan amboy.Job {
	output := make(chan amboy.Job)

	go func() {
		defer recovery.LogStackTraceAndContinue("download generator")

		for url := range urls {
			j, err := recall.NewDownloadJob(url, path, true)
			if err != nil {
				catcher.Add(errors.Wrapf(err, "problem creating download job for %s", url))
				continue
			}

			output <- j
		}
		close(output)
	}()

	return output
}

func processDownloadJobs(ctx context.Context, processFile func(string) error) func(amboy.Queue) error {
	return func(q amboy.Queue) error {
		grip.Infof("waiting for %d download jobs to complete", q.Stats(ctx).Total)
		if !amboy.WaitInterval(ctx, q, time.Second) {
			return errors.New("download job timed out")
		}
		grip.Info("all download tasks complete, processing errors now")

		if err := amboy.ResolveErrors(ctx, q); err != nil {
			return errors.Wrap(err, "problem completing download jobs")
		}

		catcher := grip.NewBasicCatcher()
		for job := range q.Results(ctx) {
			catcher.Add(job.Error())
			downloadJob, ok := job.(*recall.DownloadFileJob)
			if !ok {
				catcher.Add(errors.New("problem retrieving download job from queue"))
				continue
			}
			if err := processFile(filepath.Join(downloadJob.Directory, downloadJob.FileName)); err != nil {
				catcher.Add(err)
			}
		}
		return catcher.Resolve()
	}
}

func setupDownloadJobsAsync(ctx context.Context, jobs <-chan amboy.Job, processJobs func(amboy.Queue) error) error {
	q := queue.NewLocalLimitedSize(2, 1048)
	if err := q.Start(ctx); err != nil {
		return errors.Wrap(err, "problem starting download job queue")
	}

	if err := amboy.PopulateQueue(ctx, q, jobs); err != nil {
		return errors.Wrap(err, "problem adding download jobs to queue")
	}

	go func() {
		defer recovery.LogStackTraceAndContinue("download job generator")
		if err := processJobs(q); err != nil {
			grip.Errorf(errors.Wrap(err, "error occurred while adding jobs to cache").Error())
		}
	}()

	return nil
}
