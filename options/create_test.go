package options

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func TestCreateConstructor(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		id         string
		shouldFail bool
		cmd        string
		args       []string
	}{
		{
			id:         "EmptyString",
			shouldFail: true,
		},
		{
			id:         "BasicCmd",
			args:       []string{"ls", "-lha"},
			cmd:        "ls -lha",
			shouldFail: false,
		},
		{
			id:         "SkipsCommentsAtBeginning",
			shouldFail: true,
			cmd:        "# wat",
		},
		{
			id:         "SkipsCommentsAtEnd",
			cmd:        "ls #what",
			args:       []string{"ls"},
			shouldFail: false,
		},
		{
			id:         "UnbalancedShellLex",
			cmd:        "' foo",
			shouldFail: true,
		},
	} {
		t.Run(test.id, func(t *testing.T) {
			opt, err := MakeCreation(test.cmd)
			if test.shouldFail {
				check.Error(t, err)
				check.True(t, opt == nil)
				return
			}

			check.NotError(t, err)
			check.True(t, opt != nil)
			check.EqualItems(t, test.args, opt.Args)
		})
	}
}

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for name, test := range map[string]func(t *testing.T, opts *Create){
		"DefaultConfigForTestsValidate": func(t *testing.T, opts *Create) {
			check.NotError(t, opts.Validate())
		},
		"EmptyArgsShouldNotValidate": func(t *testing.T, opts *Create) {
			opts.Args = []string{}
			check.Error(t, opts.Validate())
		},
		"ZeroTimeoutShouldNotError": func(t *testing.T, opts *Create) {
			opts.Timeout = 0
			check.NotError(t, opts.Validate())
		},
		"SmallTimeoutShouldNotValidate": func(t *testing.T, opts *Create) {
			opts.Timeout = time.Millisecond
			check.Error(t, opts.Validate())
		},
		"LargeTimeoutShouldValidate": func(t *testing.T, opts *Create) {
			opts.Timeout = time.Hour
			check.NotError(t, opts.Validate())
		},
		"StandardInputBytesSetsStandardInput": func(t *testing.T, opts *Create) {
			stdinBytesStr := "foo"
			opts.StandardInputBytes = []byte(stdinBytesStr)

			require.NoError(t, opts.Validate())

			out, err := io.ReadAll(opts.StandardInput)
			require.NoError(t, err)
			check.Equal(t, stdinBytesStr, string(out))
		},
		"StandardInputBytesTakePrecedenceOverStandardInput": func(t *testing.T, opts *Create) {
			stdinStr := "foo"
			opts.StandardInput = bytes.NewBufferString(stdinStr)

			stdinBytesStr := "bar"
			opts.StandardInputBytes = []byte(stdinBytesStr)

			require.NoError(t, opts.Validate())

			out, err := io.ReadAll(opts.StandardInput)
			require.NoError(t, err)
			check.Equal(t, stdinBytesStr, string(out))
		},
		"NonExistingWorkingDirectoryShouldNotValidate": func(t *testing.T, opts *Create) {
			opts.WorkingDirectory = "foo"
			check.Error(t, opts.Validate())
		},
		"ExtantWorkingDirectoryShouldPass": func(t *testing.T, opts *Create) {
			wd, err := os.Getwd()
			check.NotError(t, err)
			check.NotZero(t, wd)

			opts.WorkingDirectory = wd
			check.NotError(t, opts.Validate())
		},
		"WorkingDirectoryShouldErrorForFiles": func(t *testing.T, opts *Create) {
			gobin, err := exec.LookPath("go")
			check.NotError(t, err)
			check.NotZero(t, gobin)

			opts.WorkingDirectory = gobin
			check.Error(t, opts.Validate())
		},
		"MustSpecifyValidOutput": func(t *testing.T, opts *Create) {
			opts.Output.SendErrorToOutput = true
			opts.Output.SendOutputToError = true
			check.Error(t, opts.Validate())
		},
		"WorkingDirectoryUnresolveableShouldNotError": func(t *testing.T, opts *Create) {
			cmd, _, err := opts.Resolve(ctx)
			require.NoError(t, err)
			require.NotNil(t, cmd)
			check.NotZero(t, cmd.Dir())
			check.Equal(t, opts.WorkingDirectory, cmd.Dir())
		},
		"ResolveFailsIfOptionsAreFatal": func(t *testing.T, opts *Create) {
			opts.Args = []string{}
			cmd, _, err := opts.Resolve(ctx)
			check.Error(t, err)
			check.True(t, cmd == nil)
		},
		"WithoutOverrideEnvironmentEnvIsPopulated": func(t *testing.T, opts *Create) {
			cmd, _, err := opts.Resolve(ctx)
			check.NotError(t, err)
			check.True(t, len(cmd.Env()) != 0)
		},
		"WithOverrideEnvironmentEnvIsEmpty": func(t *testing.T, opts *Create) {
			opts.OverrideEnviron = true
			cmd, _, err := opts.Resolve(ctx)
			check.NotError(t, err)
			check.Equal(t, len(cmd.Env()), 0)
		},
		"EnvironmentVariablesArePropagated": func(t *testing.T, opts *Create) {
			opts.Environment = map[string]string{
				"foo": "bar",
			}

			cmd, _, err := opts.Resolve(ctx)
			check.NotError(t, err)
			check.Contains(t, cmd.Env(), "foo=bar")
			check.NotContains(t, cmd.Env(), "bar=foo")
		},
		"MultipleArgsArePropagated": func(t *testing.T, opts *Create) {
			opts.Args = append(opts.Args, "-lha")
			cmd, _, err := opts.Resolve(ctx)
			check.NotError(t, err)
			require.Equal(t, len(cmd.Args()), 2)
			check.Equal(t, cmd.Args()[0], "ls")
			check.Equal(t, cmd.Args()[1], "-lha")
		},
		"WithOnlyCommandsArgsHasOneVal": func(t *testing.T, opts *Create) {
			cmd, _, err := opts.Resolve(ctx)
			check.NotError(t, err)
			require.Equal(t, len(cmd.Args()), 1)
			check.Equal(t, "ls", cmd.Args()[0])
		},
		"WithTimeout": func(t *testing.T, opts *Create) {
			opts.Timeout = time.Second
			opts.Args = []string{"sleep", "2"}

			cmd, deadline, err := opts.Resolve(ctx)
			require.NoError(t, err)
			check.True(t, time.Now().Before(deadline))
			check.NotError(t, cmd.Start())
			check.Error(t, cmd.Wait())
			check.True(t, time.Now().After(deadline))
		},
		"ReturnedContextWrapsResolveContext": func(t *testing.T, opts *Create) {
			opts.Args = []string{"sleep", "10"}
			opts.Timeout = 2 * time.Second
			tctx, tcancel := context.WithTimeout(ctx, time.Millisecond)
			defer tcancel()

			cmd, deadline, err := opts.Resolve(ctx)
			require.NoError(t, err)
			check.NotError(t, cmd.Start())
			check.Error(t, cmd.Wait())
			check.ErrorIs(t, tctx.Err(), context.DeadlineExceeded)
			check.True(t, time.Now().After(deadline))
		},
		"ReturnedContextErrorsOnTimeout": func(t *testing.T, opts *Create) {
			opts.Args = []string{"sleep", "10"}
			opts.Timeout = time.Second
			tctx, tcancel := context.WithTimeout(ctx, 5*time.Second)
			defer tcancel()

			start := time.Now()
			cmd, deadline, err := opts.Resolve(ctx)
			require.NoError(t, err)
			check.NotError(t, cmd.Start())
			check.Error(t, cmd.Wait())
			elapsed := time.Since(start)
			check.True(t, elapsed > opts.Timeout)
			check.NotError(t, tctx.Err())
			check.True(t, time.Now().After(deadline))
		},
		"ClosersAreAlwaysCalled": func(t *testing.T, opts *Create) {
			var counter int
			opts.closers = append(opts.closers,
				func() (_ error) { counter++; return },
				func() (_ error) { counter += 2; return },
			)
			check.NotError(t, opts.Close())
			check.Equal(t, counter, 3)

		},
		"ConflictingTimeoutOptions": func(t *testing.T, opts *Create) {
			opts.TimeoutSecs = 100
			opts.Timeout = time.Hour

			check.Error(t, opts.Validate())
		},
		"ValidationOverrideDefaultsForSecond": func(t *testing.T, opts *Create) {
			opts.TimeoutSecs = 100
			opts.Timeout = 0

			check.NotError(t, opts.Validate())
			check.Equal(t, 100*time.Second, opts.Timeout)
		},
		"ValidationOverrideDefaultsForDuration": func(t *testing.T, opts *Create) {
			opts.TimeoutSecs = 0
			opts.Timeout = time.Second

			check.NotError(t, opts.Validate())
			check.Equal(t, 1, opts.TimeoutSecs)
		},
	} {
		t.Run(name, func(t *testing.T) {
			opts := &Create{Args: []string{"ls"}}
			test(t, opts)
		})
	}
}

func TestFileLogging(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	badFileName := "this_does_not_exist"
	// Ensure bad file to cat doesn't exist so that command will write error message to standard error
	_, err := os.Stat(badFileName)
	require.True(t, os.IsNotExist(err))

	catOutputMessage := "foobar"
	outputSize := int64(len(catOutputMessage) + 1)
	catErrorMessage := "cat: this_does_not_exist: No such file or directory"
	errorSize := int64(len(catErrorMessage) + 1)

	// Ensure good file exists and has data
	goodFile, err := os.CreateTemp("", "this_file_exists")
	require.NoError(t, err)
	defer func() {
		check.NotError(t, goodFile.Close())
		check.NotError(t, os.RemoveAll(goodFile.Name()))
	}()

	goodFileName := goodFile.Name()
	numBytes, err := goodFile.Write([]byte(catOutputMessage))
	require.NoError(t, err)
	require.NotZero(t, numBytes)

	args := map[string][]string{
		"Output":         {"cat", goodFileName},
		"Error":          {"cat", badFileName},
		"OutputAndError": {"cat", goodFileName, badFileName},
	}

	for _, testParams := range []struct {
		id               string
		command          []string
		numBytesExpected int64
		numLogs          int
		outOpts          Output
	}{
		{
			id:               "LoggerWritesOutputToOneFileEndpoint",
			command:          args["Output"],
			numBytesExpected: outputSize,
			numLogs:          1,
			outOpts:          Output{SuppressOutput: false, SuppressError: false},
		},
		{
			id:               "LoggerWritesOutputToMultipleFileEndpoints",
			command:          args["Output"],
			numBytesExpected: outputSize,
			numLogs:          2,
			outOpts:          Output{SuppressOutput: false, SuppressError: false},
		},
		{
			id:               "LoggerWritesErrorToFileEndpoint",
			command:          args["Error"],
			numBytesExpected: errorSize,
			numLogs:          1,
			outOpts:          Output{SuppressOutput: true, SuppressError: false},
		},
		// {
		// 	id:               "LoggerReadsFromBothStandardOutputAndStandardError",
		// 	command:          args["OutputAndError"],
		// 	numBytesExpected: outputSize + errorSize,
		// 	numLogs:          1,
		// 	outOpts:          Output{SuppressOutput: false, SuppressError: false},
		// },
		{
			id:               "LoggerIgnoresOutputWhenSuppressed",
			command:          args["Output"],
			numBytesExpected: 0,
			numLogs:          1,
			outOpts:          Output{SuppressOutput: true, SuppressError: false},
		},
		{
			id:               "LoggerIgnoresErrorWhenSuppressed",
			command:          args["Error"],
			numBytesExpected: 0,
			numLogs:          1,
			outOpts:          Output{SuppressOutput: false, SuppressError: true},
		},
		{
			id:               "LoggerIgnoresOutputAndErrorWhenSuppressed",
			command:          args["OutputAndError"],
			numBytesExpected: 0,
			numLogs:          1,
			outOpts:          Output{SuppressOutput: true, SuppressError: true},
		},
		{
			id:               "LoggerReadsFromRedirectedOutput",
			command:          args["Output"],
			numBytesExpected: outputSize,
			numLogs:          1,
			outOpts:          Output{SuppressOutput: false, SuppressError: false, SendOutputToError: true},
		},
		{
			id:               "LoggerReadsFromRedirectedError",
			command:          args["Error"],
			numBytesExpected: errorSize,
			numLogs:          1,
			outOpts:          Output{SuppressOutput: false, SuppressError: false, SendErrorToOutput: true},
		},
	} {
		t.Run(testParams.id, func(t *testing.T) {
			files := []*os.File{}
			for i := 0; i < testParams.numLogs; i++ {
				file, err := os.CreateTemp("", "out.txt")
				require.NoError(t, err)
				defer func() {
					check.NotError(t, file.Close())
					check.NotError(t, os.RemoveAll(file.Name()))
				}()
				info, err := file.Stat()
				require.NoError(t, err)
				check.Zero(t, info.Size())
				files = append(files, file)
			}

			opts := Create{Output: testParams.outOpts}
			for _, file := range files {
				logger := &LoggerConfig{
					info: loggerConfigInfo{
						Type:   LogFile,
						Format: RawLoggerConfigFormatJSON,
					},
					producer: &FileLoggerOptions{
						Filename: file.Name(),
						Base:     BaseOptions{Format: LogFormatPlain},
					},
				}
				opts.Output.Loggers = append(opts.Output.Loggers, logger)
			}
			opts.Args = testParams.command

			cmd, _, err := opts.Resolve(ctx)
			require.NoError(t, err)
			require.NoError(t, cmd.Start())

			_ = cmd.Wait()
			check.NotError(t, opts.Close())
			testt.Log(t, "number of files:", len(files))

			for _, file := range files {
				info, err := file.Stat()
				check.NotError(t, err)
				check.Equal(t, testParams.numBytesExpected, info.Size())
			}
		})
	}
}
