package options

import (
	"encoding/json"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

func TestLoggingCache(t *testing.T) {
	t.Run("Close", func(t *testing.T) {
		t.Run("OutputAndErrorSenders", func(t *testing.T) {
			outputSender := NewMockSender("output")
			errorSender := NewMockSender("error")

			logger := &CachedLogger{
				Output: outputSender,
				Error:  errorSender,
			}
			assert.NotError(t, logger.Close())
			check.True(t, outputSender.Closed)
			check.True(t, errorSender.Closed)
		})
		t.Run("OutputSender", func(t *testing.T) {
			outputSender := NewMockSender("output")

			logger := &CachedLogger{Output: outputSender}
			assert.NotError(t, logger.Close())
			check.True(t, outputSender.Closed)
		})
		t.Run("ErrorSender", func(t *testing.T) {
			errorSender := NewMockSender("error")

			logger := &CachedLogger{Error: errorSender}
			assert.NotError(t, logger.Close())
			check.True(t, errorSender.Closed)
		})
		t.Run("SameSender", func(t *testing.T) {
			outputSender := NewMockSender("output")

			logger := &CachedLogger{
				Output: outputSender,
				Error:  outputSender,
			}
			assert.NotError(t, logger.Close())
			check.True(t, outputSender.Closed)
		})
		t.Run("OutputSenderCloseError", func(t *testing.T) {
			outputSender := NewMockSender("output")
			outputSender.Closed = true

			logger := &CachedLogger{Output: outputSender}
			check.Error(t, logger.Close())
		})
		t.Run("ErrorSenderCloseError", func(t *testing.T) {
			errorSender := NewMockSender("output")
			errorSender.Closed = true

			logger := &CachedLogger{Error: errorSender}
			check.Error(t, logger.Close())
		})

	})
	t.Run("LoggingSendErrors", func(t *testing.T) {
		lp := &LoggingPayload{}
		cl := &CachedLogger{}
		t.Run("Unconfigured", func(t *testing.T) {
			err := cl.Send(lp)
			check.Error(t, err)
		})
		t.Run("IvalidMessage", func(t *testing.T) {
			lp.Format = LoggingPayloadFormatJSON
			lp.Data = "{hello, world!\""
			lp.Priority = level.Trace
			logger := &CachedLogger{Output: grip.Sender()}
			assert.Error(t, logger.Send(lp))
		})

	})
	t.Run("OutputTargeting", func(t *testing.T) {
		output := send.MakeInternal()
		error := send.MakeInternal()
		lp := &LoggingPayload{Data: "hello world!", Priority: level.Info}
		t.Run("Output", func(t *testing.T) {
			check.Equal(t, 0, output.Len())
			cl := &CachedLogger{Output: output}
			assert.NotError(t, cl.Send(lp))
			assert.Equal(t, 1, output.Len())
			msg := output.GetMessage()
			check.Equal(t, "hello world!", msg.Message.String())
		})
		t.Run("Error", func(t *testing.T) {
			check.Equal(t, 0, error.Len())
			cl := &CachedLogger{Error: error}
			assert.NotError(t, cl.Send(lp))
			assert.Equal(t, 1, error.Len())
			msg := error.GetMessage()
			check.Equal(t, "hello world!", msg.Message.String())
		})
		t.Run("ErrorForce", func(t *testing.T) {
			lp.PreferSendToError = true
			check.Equal(t, 0, error.Len())
			cl := &CachedLogger{Error: error, Output: output}
			assert.NotError(t, cl.Send(lp))
			assert.Equal(t, 1, error.Len())
			msg := error.GetMessage()
			check.Equal(t, "hello world!", msg.Message.String())
		})
	})
	t.Run("Messages", func(t *testing.T) {
		t.Run("SingleMessageProduction", func(t *testing.T) {
			t.Run("JSON", func(t *testing.T) {
				lp := &LoggingPayload{Format: LoggingPayloadFormatJSON}

				t.Run("Invalid", func(t *testing.T) {
					_, err := lp.produceMessage([]byte("hello world! 42!"))
					assert.Error(t, err)
				})
				t.Run("Valid", func(t *testing.T) {
					msg, err := lp.produceMessage([]byte(`{"msg":"hello world!"}`))
					assert.NotError(t, err)
					assert.Equal(t, `msg='hello world!'`, msg.String())
					raw, err := json.Marshal(msg.Raw())
					assert.NotError(t, err)
					assert.Equal(t, len(raw), 22)
				})
				t.Run("ValidMetadata", func(t *testing.T) {
					lp.AddMetadata = true
					msg, err := lp.produceMessage([]byte(`{"msg":"hello world!"}`))
					assert.NotError(t, err)
					assert.Equal(t, `msg='hello world!'`, msg.String())
					raw, err := json.Marshal(msg.Raw())
					testt.Log(t, len(raw))
					assert.NotError(t, err)
					assert.Equal(t, len(raw), 127)
					check.Substring(t, string(raw), "proc")
					check.Substring(t, string(raw), "host")
					check.Substring(t, string(raw), "meta")
				})
			})
			t.Run("String", func(t *testing.T) {
				lp := &LoggingPayload{Format: LoggingPayloadFormatSTRING}

				msg, err := lp.produceMessage([]byte("hello world! 42!"))
				assert.NotError(t, err)
				assert.Equal(t, "hello world! 42!", msg.String())
				t.Run("Raw", func(t *testing.T) {
					raw, err := json.Marshal(msg.Raw())
					assert.NotError(t, err)
					assert.True(t, len(raw) >= 24)
				})
				t.Run("WithMetadata", func(t *testing.T) {
					lp.AddMetadata = true

					msg, err := lp.produceMessage([]byte("hello world! 42!"))
					assert.NotError(t, err)
					assert.Equal(t, "hello world! 42!", msg.String())
					raw, err := json.Marshal(msg.Raw())
					assert.NotError(t, err)
					testt.Logf(t, "%d:%s", len(raw), string(raw))
					assert.True(t, len(raw) >= 50)
				})
			})
		})
		t.Run("ConvertSingle", func(t *testing.T) {
			lp := &LoggingPayload{}
			t.Run("String", func(t *testing.T) {
				msg, err := lp.convertMessage("hello world")
				assert.NotError(t, err)
				assert.Equal(t, "hello world", msg.String())
			})
			t.Run("ByteSlice", func(t *testing.T) {
				msg, err := lp.convertMessage([]byte("hello world"))
				assert.NotError(t, err)
				assert.Equal(t, "hello world", msg.String())
			})
			t.Run("StringSlice", func(t *testing.T) {
				msg, err := lp.convertMessage([]string{"hello", "world"})
				assert.NotError(t, err)
				assert.Equal(t, "hello world", msg.String())
			})
			t.Run("MultiByteSlice", func(t *testing.T) {
				msg, err := lp.convertMessage([][]byte{[]byte("hello"), []byte("world")})
				assert.NotError(t, err)
				assert.Equal(t, "hello\nworld", msg.String())
			})
			t.Run("InterfaceSlice", func(t *testing.T) {
				msg, err := lp.convertMessage([]interface{}{"hello", true, "world", 42})
				assert.NotError(t, err)
				assert.Equal(t, "hello='true' world='42'", msg.String())
			})
			t.Run("Interface", func(t *testing.T) {
				msg, err := lp.convertMessage(ex{})
				assert.NotError(t, err)
				assert.Equal(t, "hello world!", msg.String())
			})
			t.Run("Composer", func(t *testing.T) {
				msg, err := lp.convertMessage(message.MakeString("jasper"))
				assert.NotError(t, err)
				assert.Equal(t, "jasper", msg.String())
			})
		})
		t.Run("ConvertMulti", func(t *testing.T) {
			lp := &LoggingPayload{}
			t.Run("String", func(t *testing.T) {
				t.Run("Single", func(t *testing.T) {
					msg, err := lp.convertMultiMessage("hello world")
					assert.NotError(t, err)
					assert.Equal(t, "hello world", msg.String())
				})
				t.Run("Many", func(t *testing.T) {
					msg, err := lp.convertMultiMessage("hello\nworld")
					assert.NotError(t, err)
					group := requireGroup(t, 2, msg)
					check.Equal(t, "hello", group[0].String())
					check.Equal(t, "world", group[1].String())
				})
			})
			t.Run("Byte", func(t *testing.T) {
				t.Run("Strings", func(t *testing.T) {
					msg, err := lp.convertMultiMessage([]byte("hello\x00world"))
					assert.NotError(t, err)
					group := requireGroup(t, 2, msg)
					check.Equal(t, "hello", group[0].String())
					check.Equal(t, "world", group[1].String())

				})
			})
			t.Run("InterfaceSlice", func(t *testing.T) {
				msg, err := lp.convertMultiMessage([]interface{}{"hello", true, "world", 42})
				assert.NotError(t, err)
				msgs := requireGroup(t, 4, msg)
				check.Equal(t, "hello", msgs[0].String())
				check.Equal(t, "true", msgs[1].String())
				check.Equal(t, "42", msgs[3].String())
			})
			t.Run("Composers", func(t *testing.T) {
				msg, err := lp.convertMultiMessage([]message.Composer{
					message.MakeString("hello world"),
					message.MakeString("jasper"),
					message.MakeString("grip"),
				})
				assert.NotError(t, err)
				msgs := requireGroup(t, 3, msg)
				check.Equal(t, "hello world", msgs[0].String())
				check.Equal(t, "grip", msgs[2].String())
			})

		})
		t.Run("ConvertMultiDetection", func(t *testing.T) {
			lp := &LoggingPayload{Data: []string{"hello", "world"}}
			t.Run("Multi", func(t *testing.T) {
				lp.IsMulti = true
				msg, err := lp.convert()
				assert.NotError(t, err)
				msgs := requireGroup(t, 2, msg)

				check.Equal(t, "hello", msgs[0].String())
				check.Equal(t, "world", msgs[1].String())
			})
			t.Run("Single", func(t *testing.T) {
				lp.IsMulti = false
				msg, err := lp.convert()
				assert.NotError(t, err)
				_, ok := msg.(*message.GroupComposer)
				check.True(t, !ok)
				check.Equal(t, "hello world", msg.String())
			})
		})
	})
}

type ex struct{}

func (ex) String() string { return "hello world!" }

func requireGroup(t *testing.T, size int, msg message.Composer) []message.Composer {
	t.Helper()
	t.Logf("%T", msg)
	gc, ok := msg.(*message.GroupComposer)
	assert.True(t, ok)
	msgs := gc.Messages()
	assert.Equal(t, len(msgs), size)
	return msgs
}
