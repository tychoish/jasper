package options

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
	"github.com/tychoish/grip/send"
)

func TestOutputOptions(t *testing.T) {
	stdout := bytes.NewBuffer([]byte{})
	stderr := bytes.NewBuffer([]byte{})

	type testCase func(*testing.T, Output)

	cases := map[string]testCase{
		"NilOptionsValidate": func(t *testing.T, opts Output) {
			check.True(t, opts.Output == nil)
			check.True(t, opts.Error == nil)
			check.NotError(t, opts.Validate())
		},
		"ErrorOutputSpecified": func(t *testing.T, opts Output) {
			opts.Output = stdout
			opts.Error = stderr
			check.NotError(t, opts.Validate())
		},
		"SuppressErrorWhenSpecified": func(t *testing.T, opts Output) {
			opts.Error = stderr
			opts.SuppressError = true
			check.Error(t, opts.Validate())
		},
		"SuppressOutputWhenSpecified": func(t *testing.T, opts Output) {
			opts.Output = stdout
			opts.SuppressOutput = true
			check.Error(t, opts.Validate())
		},
		"RedirectErrorToNillFails": func(t *testing.T, opts Output) {
			opts.SendOutputToError = true
			check.Error(t, opts.Validate())
		},
		"RedirectOutputToError": func(t *testing.T, opts Output) {
			opts.SendOutputToError = true
			check.Error(t, opts.Validate())
		},
		"SuppressAndRedirectOutputIsInvalid": func(t *testing.T, opts Output) {
			opts.SuppressOutput = true
			opts.SendOutputToError = true
			check.Error(t, opts.Validate())
		},
		"SuppressAndRedirectErrorIsInvalid": func(t *testing.T, opts Output) {
			opts.SuppressError = true
			opts.SendErrorToOutput = true
			check.Error(t, opts.Validate())
		},
		"DiscardIsNilForOutput": func(t *testing.T, opts Output) {
			opts.Error = stderr
			opts.Output = io.Discard

			check.True(t, opts.outputIsNull())
			check.True(t, !opts.errorIsNull())
		},
		"NilForOutputIsValid": func(t *testing.T, opts Output) {
			opts.Error = stderr
			check.True(t, opts.outputIsNull())
			check.True(t, !opts.errorIsNull())
		},
		"DiscardIsNilForError": func(t *testing.T, opts Output) {
			opts.Error = io.Discard
			opts.Output = stdout
			check.True(t, opts.errorIsNull())
			check.True(t, !opts.outputIsNull())
		},
		"NilForErrorIsValid": func(t *testing.T, opts Output) {
			opts.Output = stdout
			check.True(t, opts.errorIsNull())
			check.True(t, !opts.outputIsNull())
		},
		"OutputGetterNilIsIoDiscard": func(t *testing.T, opts Output) {
			out, err := opts.GetOutput()
			check.NotError(t, err)
			check.True(t, io.Discard == out)
		},
		"OutputGetterWhenPopulatedIsCorrect": func(t *testing.T, opts Output) {
			opts.Output = stdout
			out, err := opts.GetOutput()
			check.NotError(t, err)
			check.True(t, stdout == out)
		},
		"ErrorGetterNilIsIoDiscard": func(t *testing.T, opts Output) {
			outErr, err := opts.GetError()
			check.NotError(t, err)
			check.True(t, io.Discard == outErr)
		},
		"ErrorGetterWhenPopulatedIsCorrect": func(t *testing.T, opts Output) {
			opts.Error = stderr
			outErr, err := opts.GetError()
			check.NotError(t, err)
			check.True(t, stderr == outErr)
		},
		"RedirectErrorHasCorrectSemantics": func(t *testing.T, opts Output) {
			opts.Output = stdout
			opts.Error = stderr
			opts.SendErrorToOutput = true
			outErr, err := opts.GetError()
			check.NotError(t, err)
			check.True(t, stdout == outErr)
		},
		"RedirectOutputHasCorrectSemantics": func(t *testing.T, opts Output) {
			opts.Output = stdout
			opts.Error = stderr
			opts.SendOutputToError = true
			out, err := opts.GetOutput()
			check.NotError(t, err)
			check.True(t, stderr == out)
		},
		"RedirectCannotHaveCycle": func(t *testing.T, opts Output) {
			opts.Output = stdout
			opts.Error = stderr
			opts.SendOutputToError = true
			opts.SendErrorToOutput = true
			check.Error(t, opts.Validate())
		},
		"SuppressOutputWithLogger": func(t *testing.T, opts Output) {
			opts.Loggers = []*LoggerConfig{
				{
					info: loggerConfigInfo{
						Type:   LogDefault,
						Format: RawLoggerConfigFormatBSON,
					},
				},
			}
			opts.SuppressOutput = true
			check.NotError(t, opts.Validate())
		},
		"SuppressErrorWithLogger": func(t *testing.T, opts Output) {
			opts.Loggers = []*LoggerConfig{
				{
					info: loggerConfigInfo{
						Type:   LogDefault,
						Format: RawLoggerConfigFormatBSON,
					},
				},
			}
			opts.SuppressError = true
			check.NotError(t, opts.Validate())
		},
		"SuppressOutputAndErrorWithLogger": func(t *testing.T, opts Output) {
			opts.Loggers = []*LoggerConfig{
				{
					info: loggerConfigInfo{
						Type:   LogDefault,
						Format: RawLoggerConfigFormatBSON,
					},
				},
			}
			opts.SuppressOutput = true
			opts.SuppressError = true
			check.NotError(t, opts.Validate())
		},
		"RedirectOutputWithLogger": func(t *testing.T, opts Output) {
			opts.Loggers = []*LoggerConfig{
				{
					info: loggerConfigInfo{
						Type:   LogDefault,
						Format: RawLoggerConfigFormatBSON,
					},
				},
			}
			opts.SendOutputToError = true
			check.NotError(t, opts.Validate())
		},
		"RedirectErrorWithLogger": func(t *testing.T, opts Output) {
			opts.Loggers = []*LoggerConfig{
				{
					info: loggerConfigInfo{
						Type:   LogDefault,
						Format: RawLoggerConfigFormatBSON,
					},
				},
			}
			opts.SendErrorToOutput = true
			check.NotError(t, opts.Validate())
		},
		"GetOutputWithStdoutAndLogger": func(t *testing.T, opts Output) {
			opts.Output = stdout
			opts.Loggers = []*LoggerConfig{
				{
					info: loggerConfigInfo{
						Type:   LogInMemory,
						Format: RawLoggerConfigFormatBSON,
					},
					producer: &InMemoryLoggerOptions{
						InMemoryCap: 100,
						Base:        BaseOptions{Format: LogFormatPlain},
					},
				},
			}
			out, err := opts.GetOutput()
			require.NoError(t, err)

			msg := "foo"
			_, err = out.Write([]byte(msg))
			check.NotError(t, err)
			check.NotError(t, opts.outputSender.Close())

			check.Equal(t, msg, stdout.String())

			safeSender, ok := opts.Loggers[0].sender.(*SafeSender)
			require.True(t, ok)
			sender, ok := safeSender.Sender.(*send.InMemorySender)
			require.True(t, ok)

			logOut, err := sender.GetString()
			require.NoError(t, err)
			require.Equal(t, 1, len(logOut))
			check.Equal(t, msg, strings.Join(logOut, ""))
		},
		"GetErrorWithErrorAndLogger": func(t *testing.T, opts Output) {
			opts.Error = stderr
			opts.Loggers = []*LoggerConfig{
				{
					info: loggerConfigInfo{
						Type:   LogInMemory,
						Format: RawLoggerConfigFormatJSON,
					},
					producer: &InMemoryLoggerOptions{
						InMemoryCap: 100,
						Base:        BaseOptions{Format: LogFormatPlain},
					},
				},
			}
			errOut, err := opts.GetError()
			require.NoError(t, err)

			msg := "foo"
			_, err = errOut.Write([]byte(msg))
			check.NotError(t, err)
			check.NotError(t, opts.errorSender.Close())

			check.Equal(t, msg, stderr.String())

			safeSender, ok := opts.Loggers[0].sender.(*SafeSender)
			require.True(t, ok)
			sender, ok := safeSender.Sender.(*send.InMemorySender)
			require.True(t, ok)

			logErr, err := sender.GetString()
			require.NoError(t, err)
			require.Equal(t, 1, len(logErr))
			check.Equal(t, msg, strings.Join(logErr, ""))
		},
		// "": func(t *testing.T, opts Output) {}
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			test(t, Output{})
		})
	}
}

func TestOutputIntegrationTableTest(t *testing.T) {
	buf := &bytes.Buffer{}
	shouldFail := []Output{
		{Output: buf, SendOutputToError: true},
	}

	shouldPass := []Output{
		{Output: buf, Error: buf},
		{SuppressError: true, SuppressOutput: true},
		{Output: buf, SendErrorToOutput: true},
	}

	for _, opt := range shouldFail {
		check.Error(t, opt.Validate())
	}

	for idx, opt := range shouldPass {
		testt.Logf(t, "%d: %+v", idx, opt)
		check.NotError(t, opt.Validate())
	}

}
