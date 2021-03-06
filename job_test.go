package jasper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tychoish/amboy/registry"
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

		RegisterJobs(newBasicProcess)
		assert.Equal(t, 4, numJobs())
	})
	t.Run("TypeCheck", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			job, ok := NewJob(newBasicProcess, "ls").(*amboyJob)
			assert.True(t, ok)
			assert.NotNil(t, job)
		})
		t.Run("DefaultBasic", func(t *testing.T) {
			job, ok := NewJobBasic("ls").(*amboyJob)
			assert.True(t, ok)
			assert.NotNil(t, job)
		})
		t.Run("Simple", func(t *testing.T) {
			job, ok := NewJobOptions(newBasicProcess, &options.Create{}).(*amboySimpleCapturedOutputJob)
			assert.True(t, ok)
			assert.NotNil(t, job)
		})
		t.Run("Foreground", func(t *testing.T) {
			job, ok := NewJobForeground(newBasicProcess, &options.Create{}).(*amboyForegroundOutputJob)
			assert.True(t, ok)
			assert.NotNil(t, job)
		})
		t.Run("ForegroundBasic", func(t *testing.T) {
			job, ok := NewJobBasicForeground(&options.Create{}).(*amboyForegroundOutputJob)
			assert.True(t, ok)
			assert.NotNil(t, job)
		})
	})
	t.Run("BasicExec", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			job := NewJob(newBasicProcess, "ls")
			job.Run(ctx)
			require.NoError(t, job.Error())
		})
		t.Run("DefaultBasic", func(t *testing.T) {
			job := NewJobBasic("ls")
			job.Run(ctx)
			require.NoError(t, job.Error())
		})
		t.Run("Simple", func(t *testing.T) {
			job := NewJobOptions(newBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			require.NoError(t, job.Error())
		})
		t.Run("Foreground", func(t *testing.T) {
			job := NewJobForeground(newBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			require.NoError(t, job.Error())
		})
	})
	t.Run("ReExecErrors", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			job := NewJob(newBasicProcess, "ls")
			job.Run(ctx)
			require.NoError(t, job.Error())
			job.Run(ctx)
			require.Error(t, job.Error())
		})
		t.Run("Simple", func(t *testing.T) {
			job := NewJobOptions(newBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			require.NoError(t, job.Error())
			job.Run(ctx)
			require.Error(t, job.Error())
		})
		t.Run("Foreground", func(t *testing.T) {
			job := NewJobForeground(newBasicProcess, &options.Create{Args: []string{"echo", "hi"}})
			job.Run(ctx)
			require.NoError(t, job.Error())
			job.Run(ctx)
			require.Error(t, job.Error())
		})
	})
}
