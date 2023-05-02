package jamboy

import (
	"context"
	"testing"

	"github.com/tychoish/amboy/registry"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

func TestAmboyJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Registry", func(t *testing.T) {
		numJobs := func() int {
			count := 0
			for n := range registry.JobTypeNames() {
				if n != "" {
					count++
				}
			}
			return count
		}

		RegisterJobs(jasper.NewBasicProcess)
		check.Equal(t, 4, numJobs())
	})
	t.Run("TypeCheck", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			job, ok := NewJob(jasper.NewBasicProcess, "ls").(*amboyJob)
			check.True(t, ok)
			check.NotZero(t, job)
		})
		t.Run("DefaultBasic", func(t *testing.T) {
			job, ok := NewJobBasic("ls").(*amboyJob)
			check.True(t, ok)
			check.NotZero(t, job)
		})
		t.Run("Simple", func(t *testing.T) {
			job, ok := NewJobOptions(jasper.NewBasicProcess, &options.Create{}).(*amboySimpleCapturedOutputJob)
			check.True(t, ok)
			check.NotZero(t, job)
		})
		t.Run("Foreground", func(t *testing.T) {
			job, ok := NewJobForeground(jasper.NewBasicProcess, &options.Create{}).(*amboyForegroundOutputJob)
			check.True(t, ok)
			check.NotZero(t, job)
		})
		t.Run("ForegroundBasic", func(t *testing.T) {
			job, ok := NewJobBasicForeground(&options.Create{}).(*amboyForegroundOutputJob)
			check.True(t, ok)
			check.NotZero(t, job)
		})
	})
	t.Run("BasicExec", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			job := NewJob(jasper.NewBasicProcess, "ls")
			job.Run(ctx)
			assert.NotError(t, job.Error())
		})
		t.Run("DefaultBasic", func(t *testing.T) {
			job := NewJobBasic("ls")
			job.Run(ctx)
			assert.NotError(t, job.Error())
		})
		t.Run("Simple", func(t *testing.T) {
			job := NewJobOptions(jasper.NewBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			assert.NotError(t, job.Error())
		})
		t.Run("Foreground", func(t *testing.T) {
			job := NewJobForeground(jasper.NewBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			assert.NotError(t, job.Error())
		})
	})
	t.Run("ReExecErrors", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			job := NewJob(jasper.NewBasicProcess, "ls")
			job.Run(ctx)
			assert.NotError(t, job.Error())
			job.Run(ctx)
			assert.Error(t, job.Error())
		})
		t.Run("Simple", func(t *testing.T) {
			job := NewJobOptions(jasper.NewBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			assert.NotError(t, job.Error())
			job.Run(ctx)
			assert.Error(t, job.Error())
		})
		t.Run("Foreground", func(t *testing.T) {
			job := NewJobForeground(jasper.NewBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			assert.NotError(t, job.Error())
			job.Run(ctx)
			assert.Error(t, job.Error())
		})
	})

}
