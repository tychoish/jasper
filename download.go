package jasper

import (
	"context"
	"path/filepath"
	"time"

	"github.com/cdr/amboy"
	"github.com/deciduosity/bond/recall"
	"github.com/cdr/grip"
	"github.com/cdr/grip/recovery"
	"github.com/pkg/errors"
)

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
		for job := range q.Jobs(ctx) {
			if !job.Status().Completed {
				continue
			}
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
