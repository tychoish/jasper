package cli

import (
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/jasper"
)

func TestExtractResponse(t *testing.T) {
	const (
		errMsg = "fail"
		s1     = "foo"
		s2     = "bar"
		n1     = 1
	)
	for outcomeName, outcome := range map[string]OutcomeResponse{
		"Success": {
			Success: true,
		},
		"Unsuccessful": {
			Success: false,
			Message: errMsg,
		},
		"UnsuccessfulDefaultError": {
			Success: false,
		},
	} {
		t.Run(outcomeName, func(t *testing.T) {
			for testName, testCase := range map[string]struct {
				input           string
				extractAndCheck func(*testing.T, []byte)
			}{
				"OperationOutcome": {
					input: fmt.Sprintf(`{
						"success": %t,
						"message": "%s"
					}`, outcome.Success, outcome.Message),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractOutcomeResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}
					},
				},
				"InfoResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"info": {
						"id": "%s"
					}
					}`, outcome.Success, outcome.Message, s1),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractInfoResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						check.Equal(t, s1, resp.Info.ID)
					},
				},
				"InfosResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"infos": [{
						"id": "%s"
					}, {
						"id": "%s"
					}]
					}`, outcome.Success, outcome.Message, s1, s2),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractInfosResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						info1, info2 := jasper.ProcessInfo{ID: s1}, jasper.ProcessInfo{ID: s2}
						info1Found, info2Found := false, false
						for _, info := range resp.Infos {
							if info.ID == info1.ID {
								info1Found = true
							}
							if info.ID == info2.ID {
								info2Found = true
							}
						}
						check.True(t, info1Found && info2Found)
					},
				},
				"TagsResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"tags": ["%s", "%s"]
					}`, outcome.Success, outcome.Message, s1, s2),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractTagsResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						check.Contains(t, resp.Tags, s1)
						check.Contains(t, resp.Tags, s2)
					},
				},
				"WaitResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"exit_code": %d,
					"error": "%s"
					}`, outcome.Success, outcome.Message, n1, errMsg),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractWaitResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
							check.Equal(t, n1, resp.ExitCode)
							check.Substring(t, resp.Error, errMsg)
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}
					},
				},
				"RunningResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"running": %t
					}`, outcome.Success, outcome.Message, true),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractRunningResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						check.True(t, resp.Running)
					},
				},
				"CompleteResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"complete": %t
					}`, outcome.Success, outcome.Message, true),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractCompleteResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						check.True(t, resp.Complete)
					},
				},
				"ServiceStatusResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"status": "%s"
					}`, outcome.Success, outcome.Message, ServiceRunning),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractServiceStatusResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						check.Equal(t, ServiceRunning, resp.Status)
					},
				},
				"LogStreamResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"log_stream": {
						"logs": ["%s"],
						"done": %t
					}
					}`, outcome.Success, outcome.Message, "foo", true),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractLogStreamResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						assert.Equal(t, len(resp.LogStream.Logs), 1)
						check.Equal(t, "foo", resp.LogStream.Logs[0])
						check.True(t, resp.LogStream.Done)
					},
				},
				"BuildloggerURLsResponse": {
					input: fmt.Sprintf(`{
					"outcome": {
						"success": %t,
						"message": "%s"
					},
					"urls": ["%s"]
					}`, outcome.Success, outcome.Message, "foo"),
					extractAndCheck: func(t *testing.T, input []byte) {
						resp, err := ExtractBuildloggerURLsResponse(input)
						if outcome.Success {
							assert.NotError(t, err)
							check.True(t, resp.Successful())
						} else {
							assert.Error(t, err)
							check.True(t, !resp.Successful())

							if outcome.Message != "" {
								check.Substring(t, resp.ErrorMessage(), outcome.Message)
							} else {
								check.Substring(t, resp.ErrorMessage(), unspecifiedRequestFailure)
							}
						}

						assert.Equal(t, len(resp.URLs), 1)
						check.Equal(t, "foo", resp.URLs[0])
					},
				},
			} {
				t.Run(testName, func(t *testing.T) {
					testCase.extractAndCheck(t, []byte(testCase.input))
				})
			}
		})
	}
}
