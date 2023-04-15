package jamboy

import (
	"context"

	"github.com/tychoish/amboy"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper"
)

// EnqueueForeground adds separate jobs to the queue for every operation
// captured in the command. These operations will execute in
// parallel. The output of the commands are logged, using the default
// grip sender in the foreground.
func EnqueueForeground(ctx context.Context, c *jasper.Command, q amboy.Queue) error {
	jobs, err := JobsForeground(c)
	if err != nil {
		return err
	}

	catcher := &erc.Collector{}
	for _, j := range jobs {
		catcher.Add(q.Put(ctx, j))
	}

	return catcher.Resolve()
}

// Enqueue adds separate jobs to the queue for every operation
// captured in the command. These operations will execute in
// parallel. The output of the operations is captured in the body of
// the job.
func Enqueue(ctx context.Context, c *jasper.Command, q amboy.Queue) error {
	jobs, err := Jobs(c)
	if err != nil {
		return err
	}

	catcher := &erc.Collector{}
	for _, j := range jobs {
		catcher.Add(q.Put(ctx, j))
	}

	return catcher.Resolve()
}

// JobsForeground returns a slice of jobs for every operation
// captured in the command. The output of the commands are logged,
// using the default grip sender in the foreground.
func JobsForeground(c *jasper.Command) ([]amboy.Job, error) {
	opts, err := c.ExportCreateOptions()
	if err != nil {
		return nil, err
	}

	out := make([]amboy.Job, len(opts))
	for idx := range opts {
		out[idx] = NewJobForeground(c.GetProcessConstructor(), opts[idx])
	}
	return out, nil
}

// Jobs returns a slice of jobs for every operation in the
// command. The output of the commands are captured in the body of the
// job.
func Jobs(c *jasper.Command) ([]amboy.Job, error) {
	opts, err := c.ExportCreateOptions()
	if err != nil {
		return nil, err
	}

	out := make([]amboy.Job, len(opts))
	for idx := range opts {
		out[idx] = NewJobOptions(c.GetProcessConstructor(), opts[idx])
	}
	return out, nil
}
