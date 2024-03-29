package jasper

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/testutil"
	"github.com/tychoish/jasper/util"
)

const (
	echo, ls         = "echo", "ls"
	arg1, arg2, arg3 = "ZXZlcmdyZWVu", "aXM=", "c28gY29vbCE="
	lsErrorMsg       = "No such file or directory"
)

func verifyCommandAndGetOutput(ctx context.Context, t *testing.T, cmd *Command, run cmdRunFunc, success bool) string {
	var buf bytes.Buffer
	bufCloser := util.NewLocalBuffer(buf)

	cmd.SetCombinedWriter(bufCloser)

	if success {
		check.NotError(t, run(cmd, ctx))
	} else {
		check.Error(t, run(cmd, ctx))
	}

	return bufCloser.String()
}

func checkOutput(t *testing.T, exists bool, output string, expectedOutputs ...string) {
	for _, expected := range expectedOutputs {
		check.True(t, exists == strings.Contains(output, expected))
	}
}

type cmdRunFunc func(*Command, context.Context) error

func TestCommandImplementation(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NotError(t, err)
	for procType, makep := range map[string]ProcessConstructor{
		"BlockingNoLock": NewBasicProcess,
		"BlockingLock":   makeLockingProcess(NewBasicProcess),
		"BasicNoLock":    NewBasicProcess,
		"BasicLock":      makeLockingProcess(NewBasicProcess),
	} {
		t.Run(procType, func(t *testing.T) {
			for runFuncType, runFunc := range map[string]cmdRunFunc{
				"NonParallel": (*Command).Run,
				"Parallel":    (*Command).RunParallel,
			} {
				t.Run(runFuncType, func(t *testing.T) {
					for name, testCase := range map[string]func(context.Context, *testing.T, Command){
						"ValidRunCommandDoesNotError": func(ctx context.Context, t *testing.T, cmd Command) {
							cmd.ID(t.Name()).Priority(level.Info).Add(
								[]string{echo, arg1},
							).Directory(cwd)
							check.NotError(t, runFunc(&cmd, ctx))
						},
						"UnsuccessfulRunCommandErrors": func(ctx context.Context, t *testing.T, cmd Command) {
							cmd.ID(t.Name()).Priority(level.Info).Add(
								[]string{ls, arg2},
							).Directory(cwd)
							err := runFunc(&cmd, ctx)
							check.Error(t, err)
							check.True(t, strings.Contains(err.Error(), "exit status"))
						},
						"WaitErrorsIfNeverStarted": func(ctx context.Context, t *testing.T, cmd Command) {
							_, err := cmd.Wait(ctx)
							check.Error(t, err)
						},
						"WaitErrorsIfProcessErrors": func(ctx context.Context, t *testing.T, cmd Command) {
							cmd.Append("false")
							check.Error(t, runFunc(&cmd, ctx))
							exitCode, err := cmd.Wait(ctx)
							check.Error(t, err)
							check.NotZero(t, exitCode)
						},
						"WaitOnBackgroundRunWaitsForProcessCompletion": func(ctx context.Context, t *testing.T, cmd Command) {
							cmd.Append("sleep 1", "sleep 1").Background(true)
							assert.NotError(t, runFunc(&cmd, ctx))
							exitCode, err := cmd.Wait(ctx)
							check.NotError(t, err)
							check.Zero(t, exitCode)

							for _, proc := range cmd.procs {
								check.True(t, proc.Info(ctx).Complete)
								check.True(t, proc.Info(ctx).Successful)
							}
						},
						"SudoFunctions": func(ctx context.Context, t *testing.T, cmd Command) {
							user := "user"
							sudoUser := "root"

							sudoCmd := "sudo"
							sudoAsCmd := fmt.Sprintf("sudo -u %s", sudoUser)

							cmd1 := strings.Join([]string{echo, arg1}, " ")
							cmd2 := strings.Join([]string{echo, arg2}, " ")

							for commandType, isRemote := range map[string]bool{
								"Remote": true,
								"Local":  false,
							} {
								t.Run(commandType, func(t *testing.T) {
									for subTestName, subTestCase := range map[string]func(ctx context.Context, t *testing.T, cmd Command){
										"VerifySudoCmd": func(ctx context.Context, t *testing.T, cmd Command) {
											cmd.Sudo(true)
											check.Equal(t, strings.Join(cmd.sudoCmd(), " "), sudoCmd)
										},
										"VerifySudoCmdWithUser": func(ctx context.Context, t *testing.T, cmd Command) {
											cmd.SudoAs(sudoUser)
											check.Equal(t, strings.Join(cmd.sudoCmd(), " "), sudoAsCmd)
										},
										"NoSudo": func(ctx context.Context, t *testing.T, cmd Command) {
											cmd.Append(cmd1)

											allOpts, err := cmd.ExportCreateOptions()
											assert.NotError(t, err)
											assert.Equal(t, len(allOpts), 1)
											args := strings.Join(allOpts[0].Args, " ")

											check.NotSubstring(t, args, sudoCmd)
										},
										"Sudo": func(ctx context.Context, t *testing.T, cmd Command) {
											checkArgs := func(args []string, expected string) {
												argsStr := strings.Join(args, " ")
												check.Substring(t, argsStr, sudoCmd)
												check.NotSubstring(t, argsStr, sudoAsCmd)
												check.Substring(t, argsStr, expected)
											}
											cmd.Sudo(true).Append(cmd1)

											allOpts, err := cmd.ExportCreateOptions()
											assert.NotError(t, err)
											assert.Equal(t, len(allOpts), 1)
											checkArgs(allOpts[0].Args, cmd1)

											cmd.Append(cmd2)
											allOpts, err = cmd.ExportCreateOptions()
											assert.NotError(t, err)
											assert.Equal(t, len(allOpts), 2)

											checkArgs(allOpts[0].Args, cmd1)
											checkArgs(allOpts[1].Args, cmd2)
										},
										"SudoAs": func(ctx context.Context, t *testing.T, cmd Command) {
											cmd.SudoAs(sudoUser).Add([]string{echo, arg1})
											checkArgs := func(args []string, expected string) {
												argsStr := strings.Join(args, " ")
												check.Substring(t, argsStr, sudoAsCmd)
												check.Substring(t, argsStr, expected)
											}

											allOpts, err := cmd.ExportCreateOptions()
											assert.NotError(t, err)
											assert.Equal(t, len(allOpts), 1)
											checkArgs(allOpts[0].Args, cmd1)

											cmd.Add([]string{echo, arg2})
											allOpts, err = cmd.ExportCreateOptions()
											assert.NotError(t, err)
											assert.Equal(t, len(allOpts), 2)
											checkArgs(allOpts[0].Args, cmd1)
											checkArgs(allOpts[1].Args, cmd2)
										},
									} {
										t.Run(subTestName, func(t *testing.T) {
											cmd = *NewCommand().ProcConstructor(makep)
											if isRemote {
												cmd.User(user).Host("localhost").Password("password")
											}
											subTestCase(ctx, t, cmd)
										})
									}
								})
							}
						},
						"InvalidArgsCommandErrors": func(ctx context.Context, t *testing.T, cmd Command) {
							cmd.Add([]string{})
							check.Equal(t, runFunc(&cmd, ctx).Error(), "cannot have empty args")
						},
						"ZeroSubCommandsIsVacuouslySuccessful": func(ctx context.Context, t *testing.T, cmd Command) {
							check.NotError(t, runFunc(&cmd, ctx))
						},
						"PreconditionDeterminesExecution": func(ctx context.Context, t *testing.T, cmd Command) {
							for _, precondition := range []func() bool{
								func() bool {
									return true
								},
								func() bool {
									return false
								},
							} {
								t.Run(fmt.Sprintf("%tPrecondition", precondition()), func(t *testing.T) {
									cmd.Prerequisite(precondition).Add([]string{echo, arg1})
									output := verifyCommandAndGetOutput(ctx, t, &cmd, runFunc, true)
									checkOutput(t, precondition(), output, arg1)
								})
							}
						},
						"SingleInvalidSubCommandCausesTotalError": func(ctx context.Context, t *testing.T, cmd Command) {
							cmd.ID(t.Name()).Priority(level.Info).Extend(
								[][]string{
									{echo, arg1},
									{ls, arg2},
									{echo, arg3},
								},
							).Directory(cwd)
							check.Error(t, runFunc(&cmd, ctx))
						},
						"ExecutionFlags": func(ctx context.Context, t *testing.T, cmd Command) {
							numCombinations := int(math.Pow(2, 3))
							for i := 0; i < numCombinations; i++ {
								continueOnError, ignoreError, includeBadCmd := i&1 == 1, i&2 == 2, i&4 == 4

								cmd := NewCommand().Add([]string{echo, arg1}).ProcConstructor(makep)
								if includeBadCmd {
									cmd.Add([]string{ls, arg3})
								}
								cmd.Add([]string{echo, arg2})

								subTestName := fmt.Sprintf(
									"ContinueOnErrorIs%tAndIgnoreErrorIs%tAndIncludeBadCmdIs%t",
									continueOnError,
									ignoreError,
									includeBadCmd,
								)
								t.Run(subTestName, func(t *testing.T) {
									if runFuncType == "Parallel" && !continueOnError {
										t.Skip("Continue on error only applies to non parallel executions")
									}
									cmd.ContinueOnError(continueOnError).IgnoreError(ignoreError)
									successful := ignoreError || !includeBadCmd
									outputAfterLsExists := !includeBadCmd || continueOnError
									output := verifyCommandAndGetOutput(ctx, t, cmd, runFunc, successful)
									checkOutput(t, true, output, arg1)
									checkOutput(t, includeBadCmd, output, lsErrorMsg)
									checkOutput(t, outputAfterLsExists, output, arg2)
								})
							}
						},
						"CommandOutputAndErrorIsReadable": func(ctx context.Context, t *testing.T, cmd Command) {
							for subName, subTestCase := range map[string]func(context.Context, *testing.T, Command, cmdRunFunc){
								"StdOutOnly": func(ctx context.Context, t *testing.T, cmd Command, run cmdRunFunc) {
									cmd.Add([]string{echo, arg1})
									cmd.Add([]string{echo, arg2})
									output := verifyCommandAndGetOutput(ctx, t, &cmd, run, true)
									checkOutput(t, true, output, arg1, arg2)
								},
								"StdErrOnly": func(ctx context.Context, t *testing.T, cmd Command, run cmdRunFunc) {
									cmd.Add([]string{ls, arg3})
									output := verifyCommandAndGetOutput(ctx, t, &cmd, run, false)
									checkOutput(t, true, output, lsErrorMsg)
								},
								"StdOutAndStdErr": func(ctx context.Context, t *testing.T, cmd Command, run cmdRunFunc) {
									cmd.Add([]string{echo, arg1})
									cmd.Add([]string{echo, arg2})
									cmd.Add([]string{ls, arg3})
									output := verifyCommandAndGetOutput(ctx, t, &cmd, run, false)
									checkOutput(t, true, output, arg1, arg2, lsErrorMsg)
								},
							} {
								t.Run(subName, func(t *testing.T) {
									cmd = *NewCommand().ProcConstructor(makep)
									subTestCase(ctx, t, cmd, runFunc)
								})
							}
						},
						"WriterOutputAndErrorIsSettable": func(ctx context.Context, t *testing.T, cmd Command) {
							for subName, subTestCase := range map[string]func(context.Context, *testing.T, Command, *util.LocalBuffer){
								"StdOutOnly": func(ctx context.Context, t *testing.T, cmd Command, buf *util.LocalBuffer) {
									cmd.SetOutputWriter(buf)
									assert.NotError(t, runFunc(&cmd, ctx))
									checkOutput(t, true, buf.String(), arg1, arg2)
									checkOutput(t, false, buf.String(), lsErrorMsg)
								},
								"StdErrOnly": func(ctx context.Context, t *testing.T, cmd Command, buf *util.LocalBuffer) {
									cmd.SetErrorWriter(buf)
									assert.NotError(t, runFunc(&cmd, ctx))
									checkOutput(t, true, buf.String(), lsErrorMsg)
									checkOutput(t, false, buf.String(), arg1, arg2)
								},
								"StdOutAndStdErr": func(ctx context.Context, t *testing.T, cmd Command, buf *util.LocalBuffer) {
									cmd.SetCombinedWriter(buf)
									assert.NotError(t, runFunc(&cmd, ctx))
									checkOutput(t, true, buf.String(), arg1, arg2, lsErrorMsg)
								},
							} {
								t.Run(subName, func(t *testing.T) {
									cmd = *NewCommand().ProcConstructor(makep).Extend([][]string{
										{echo, arg1},
										{echo, arg2},
										{ls, arg3},
									}).ContinueOnError(true).IgnoreError(true)

									var buf bytes.Buffer
									bufCloser := util.NewLocalBuffer(buf)

									subTestCase(ctx, t, cmd, bufCloser)
								})
							}
						},
						"SenderOutputAndErrorIsSettable": func(ctx context.Context, t *testing.T, cmd Command) {
							for subName, subTestCase := range map[string]func(context.Context, *testing.T, Command, *send.InMemorySender){
								"StdOutOnly": func(ctx context.Context, t *testing.T, cmd Command, sender *send.InMemorySender) {
									cmd.SetOutputSender(cmd.opts.Priority, sender)
									assert.NotError(t, runFunc(&cmd, ctx))
									out, err := sender.GetString()
									assert.NotError(t, err)
									checkOutput(t, true, strings.Join(out, "\n"), "[p=info]:", arg1, arg2)
									checkOutput(t, false, strings.Join(out, "\n"), lsErrorMsg)
								},
								"StdErrOnly": func(ctx context.Context, t *testing.T, cmd Command, sender *send.InMemorySender) {
									cmd.SetErrorSender(cmd.opts.Priority, sender)
									assert.NotError(t, runFunc(&cmd, ctx))
									out, err := sender.GetString()
									assert.NotError(t, err)
									checkOutput(t, true, strings.Join(out, "\n"), "[p=info]:", lsErrorMsg)
									checkOutput(t, false, strings.Join(out, "\n"), arg1, arg2)
								},
								"StdOutAndStdErr": func(ctx context.Context, t *testing.T, cmd Command, sender *send.InMemorySender) {
									cmd.SetCombinedSender(cmd.opts.Priority, sender)
									assert.NotError(t, runFunc(&cmd, ctx))
									out, err := sender.GetString()
									assert.NotError(t, err)
									checkOutput(t, true, strings.Join(out, "\n"), "[p=info]:", arg1, arg2, lsErrorMsg)
								},
							} {
								t.Run(subName, func(t *testing.T) {
									cmd = *NewCommand().ProcConstructor(makep).Extend([][]string{
										{echo, arg1},
										{echo, arg2},
										{ls, arg3},
									}).ContinueOnError(true).IgnoreError(true).Priority(level.Info)

									sender, err := send.NewInMemorySender(t.Name(), cmd.opts.Priority, 100)
									assert.NotError(t, err)

									subTestCase(ctx, t, cmd, sender.(*send.InMemorySender))
								})
							}
						},
						"GetProcIDsReturnsCorrectNumberOfIDs": func(ctx context.Context, t *testing.T, cmd Command) {
							subCmds := [][]string{
								{echo, arg1},
								{echo, arg2},
								{ls, arg3},
							}
							check.NotError(t, cmd.Extend(subCmds).ContinueOnError(true).IgnoreError(true).Run(ctx))
							check.Equal(t, len(cmd.GetProcIDs()), len(subCmds))
						},
						"ApplyFromOptsUpdatesCmdCorrectly": func(ctx context.Context, t *testing.T, cmd Command) {
							opts := &options.Create{
								WorkingDirectory: cwd,
							}
							cmd.ApplyFromOpts(opts).Add([]string{ls, cwd})
							output := verifyCommandAndGetOutput(ctx, t, &cmd, runFunc, true)
							checkOutput(t, true, output, "makefile")
						},
						"SetOutputOptions": func(ctx context.Context, t *testing.T, cmd Command) {
							opts := options.Output{
								SendOutputToError: true,
							}

							check.True(t, !cmd.opts.Process.Output.SendOutputToError)
							cmd.SetOutputOptions(opts)
							check.True(t, cmd.opts.Process.Output.SendOutputToError)
						},
						"ApplyFromOptsOverridesExistingOptions": func(ctx context.Context, t *testing.T, cmd Command) {
							_ = cmd.Add([]string{echo, arg1}).Directory("bar")
							genOpts, err := cmd.ExportCreateOptions()
							assert.NotError(t, err)
							assert.Equal(t, len(genOpts), 1)
							check.Equal(t, "bar", genOpts[0].WorkingDirectory)

							opts := &options.Create{WorkingDirectory: "foo"}
							_ = cmd.ApplyFromOpts(opts)
							genOpts, err = cmd.ExportCreateOptions()
							assert.NotError(t, err)
							assert.Equal(t, len(genOpts), 1)
							check.Equal(t, opts.WorkingDirectory, genOpts[0].WorkingDirectory)
						},
						"CreateOptionsAppliedInGetCreateOptionsForLocalCommand": func(ctx context.Context, t *testing.T, cmd Command) {
							opts := &options.Create{
								WorkingDirectory: "foo",
								Environment:      map[string]string{"foo": "bar"},
							}
							args := []string{echo, arg1}
							cmd.opts.Commands = [][]string{}
							_ = cmd.ApplyFromOpts(opts).Add(args)
							genOpts, err := cmd.ExportCreateOptions()
							assert.NotError(t, err)
							assert.Equal(t, len(genOpts), 1)
							check.Equal(t, opts.WorkingDirectory, genOpts[0].WorkingDirectory)
							for k, v := range opts.Environment {
								check.Equal(t, v, genOpts[0].Environment[k])
							}
						},
						"TagFunctions": func(ctx context.Context, t *testing.T, cmd Command) {
							tags := []string{"tag0", "tag1"}
							sort.Strings(tags)
							subCmds := []string{"echo hi", "echo bye"}
							for subTestName, subTestCase := range map[string]func(ctx context.Context, t *testing.T, cmd Command){
								"SetTags": func(ctx context.Context, t *testing.T, cmd Command) {
									for _, subCmd := range subCmds {
										cmd.Append(subCmd)
									}
									cmd.SetTags(tags)
									assert.NotError(t, cmd.Run(ctx))

									check.Equal(t, len(cmd.procs), len(subCmds))
									for _, proc := range cmd.procs {
										ptags := proc.GetTags()
										sort.Strings(ptags)
										check.EqualItems(t, tags, ptags)
									}
								},
								"AppendTags": func(ctx context.Context, t *testing.T, cmd Command) {
									for _, subCmd := range subCmds {
										cmd.Append(subCmd)
									}
									cmd.AppendTags(tags...)
									assert.NotError(t, cmd.Run(ctx))
									check.Equal(t, len(cmd.procs), len(subCmds))

									for _, proc := range cmd.procs {
										ptags := proc.GetTags()
										sort.Strings(ptags)
										check.EqualItems(t, tags, ptags)
									}
								},
								"ExtendTags": func(ctx context.Context, t *testing.T, cmd Command) {
									for _, subCmd := range subCmds {
										cmd.Append(subCmd)
									}
									cmd.ExtendTags(tags)
									assert.NotError(t, cmd.Run(ctx))
									check.Equal(t, len(cmd.procs), len(subCmds))
									for _, proc := range cmd.procs {
										ptags := proc.GetTags()
										sort.Strings(ptags)
										check.EqualItems(t, tags, ptags)
									}
								},
							} {
								t.Run(subTestName, func(t *testing.T) {
									cmd = *NewCommand().ProcConstructor(makep)
									subTestCase(ctx, t, cmd)
								})
							}
						},
						"SingleArgCommandSplitsShellCommandCorrectly": func(ctx context.Context, t *testing.T, cmd Command) {
							cmd.Extend([][]string{
								{"echo hello world"},
								{"echo 'hello world'"},
								{"echo 'hello\"world\"'"},
							})

							optslist, err := cmd.Export()
							assert.NotError(t, err)

							assert.Equal(t, len(optslist), 3)
							assert.EqualItems(t, []string{"echo", "hello", "world"}, optslist[0].Args)
							assert.EqualItems(t, []string{"echo", "hello world"}, optslist[1].Args)
							assert.EqualItems(t, []string{"echo", "hello\"world\""}, optslist[2].Args)
						},
						"RunFuncReceivesPopulatedOptions": func(ctx context.Context, t *testing.T, cmd Command) {
							prio := level.Warning
							user := "user"
							runFuncCalled := false
							cmd.Add([]string{echo, arg1}).
								ContinueOnError(true).IgnoreError(true).
								Priority(prio).Background(true).
								Sudo(true).SudoAs(user).
								SetRunFunc(func(opts options.Command) error {
									runFuncCalled = true
									check.True(t, opts.ContinueOnError)
									check.True(t, opts.IgnoreError)
									check.True(t, opts.RunBackground)
									check.True(t, opts.Sudo)
									check.Equal(t, user, opts.SudoUser)
									check.Equal(t, prio, opts.Priority)
									return nil
								})
							assert.NotError(t, cmd.Run(ctx))
							check.True(t, runFuncCalled)
						},
						// "": func(ctx context.Context, t *testing.T, cmd Command) {},
					} {
						t.Run(name, func(t *testing.T) {
							ctx, cancel := context.WithTimeout(context.Background(), testutil.TestTimeout)
							defer cancel()

							cmd := NewCommand().ProcConstructor(makep)
							testCase(ctx, t, *cmd)
						})
					}
				})
			}
		})
	}
}

func TestRunParallelRunsInParallel(t *testing.T) {
	cmd := NewCommand().Extend([][]string{
		{"sleep", "3"},
		{"sleep", "3"},
		{"sleep", "3"},
	})
	maxRunTimeAllowed := 3500 * time.Millisecond // 3.5m
	cctx, cancel := context.WithTimeout(context.Background(), maxRunTimeAllowed)
	defer cancel()
	// If this does not run in parallel, the context will timeout and we will
	// get an error.
	check.NotError(t, cmd.RunParallel(cctx))
}
