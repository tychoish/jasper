package jamboy

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/tychoish/amboy"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/recovery"
)

func createDownloadJobs(path string, urls <-chan string, catcher *erc.Collector) <-chan amboy.Job {
	output := make(chan amboy.Job)

	go func() {
		defer recovery.LogStackTraceAndContinue("download generator")

		for url := range urls {
			j, err := NewDownloadJob(url, path, true)
			if err != nil {
				catcher.Push(fmt.Errorf("problem creating download job for %s: %w", url, err))
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
			return fmt.Errorf("problem completing download jobs: %w", err)
		}

		catcher := &erc.Collector{}
		for job := range q.Jobs(ctx) {
			if !job.Status().Completed {
				continue
			}
			catcher.Push(job.Error())
			downloadJob, ok := job.(*downloadFileJob)
			if !ok {
				catcher.Push(errors.New("problem retrieving download job from queue"))
				continue
			}
			if err := processFile(filepath.Join(downloadJob.Directory, downloadJob.FileName)); err != nil {
				catcher.Push(err)
			}
		}
		return catcher.Resolve()
	}
}
