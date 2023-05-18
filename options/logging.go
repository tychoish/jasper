package options

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

// CachedLogger is the cached item representing a processes normal
// output. It captures information about the cached item, as well as
// go interfaces for sending log messages.
type CachedLogger struct {
	ID       string    `bson:"id" json:"id" yaml:"id"`
	Manager  string    `bson:"manager_id" json:"manager_id" yaml:"manager_id"`
	Accessed time.Time `bson:"accessed" json:"accessed" yaml:"accessed"`

	Error  send.Sender `bson:"-" json:"-" yaml:"-"`
	Output send.Sender `bson:"-" json:"-" yaml:"-"`
}

func (cl *CachedLogger) getSender(preferError bool) (send.Sender, error) {
	if preferError && cl.Error != nil {
		return cl.Error, nil
	} else if cl.Output != nil {
		return cl.Output, nil
	} else if cl.Error != nil {
		return cl.Error, nil
	}

	return nil, errors.New("no output configured")
}

// Close closes the underlying output for the cached logger.
func (cl *CachedLogger) Close() error {
	catcher := &erc.Collector{}
	if cl.Output != nil {
		erc.Check(catcher, cl.Output.Close)
	}

	if cl.Error != nil && cl.Output != cl.Error {
		erc.Check(catcher, cl.Error.Close)
	}
	return catcher.Resolve()
}

// LoggingPayload captures the arguments to the SendMessages operation.
type LoggingPayload struct {
	LoggerID          string               `bson:"logger_id" json:"logger_id" yaml:"logger_id"`
	Data              interface{}          `bson:"data" json:"data" yaml:"data"`
	Priority          level.Priority       `bson:"priority" json:"priority" yaml:"priority"`
	IsMulti           bool                 `bson:"multi,omitempty" json:"multi,omitempty" yaml:"multi,omitempty"`
	PreferSendToError bool                 `bson:"prefer_send_to_error,omitempty" json:"prefer_send_to_error,omitempty" yaml:"prefer_send_to_error,omitempty"`
	AddMetadata       bool                 `bson:"add_metadata,omitempty" json:"add_metadata,omitempty" yaml:"add_metadata,omitempty"`
	Format            LoggingPayloadFormat `bson:"payload_format,omitempty" json:"payload_format,omitempty" yaml:"payload_format,omitempty"`
}

// LoggingPayloadFormat is an set enumerated values describing the
// formating or encoding of the payload data.
type LoggingPayloadFormat string

const (
	LoggingPayloadFormatJSON   = "json"
	LoggingPayloadFormatSTRING = "string"
)

// Validate checks that the assert. fields are populated for the payload and
// the format is valid.
func (lp *LoggingPayload) Validate() error {
	catcher := &erc.Collector{}
	erc.When(catcher, lp.Data == nil, "data cannot be empty")
	switch lp.Format {
	case "", LoggingPayloadFormatJSON, LoggingPayloadFormatSTRING:
	default:
		catcher.Add(fmt.Errorf("invalid payload format '%s'", lp.Format))
	}
	return catcher.Resolve()
}

// Send resolves a sender from the cached logger (either the error or
// output endpoint), and then sends the message from the data
// payload. This method ultimately is responsible for converting the
// payload to a message format.
func (cl *CachedLogger) Send(lp *LoggingPayload) error {
	if err := lp.Validate(); err != nil {
		return fmt.Errorf("invalid logging payload: %w", err)
	}

	sender, err := cl.getSender(lp.PreferSendToError)
	if err != nil {
		return err
	}

	msg, err := lp.convert()
	if err != nil {
		return err
	}
	msg.SetPriority(lp.Priority)
	sender.Send(msg)

	return nil
}

func (lp *LoggingPayload) convert() (message.Composer, error) {
	if lp.IsMulti {
		return lp.convertMultiMessage(lp.Data)
	}
	return lp.convertMessage(lp.Data)
}

func (lp *LoggingPayload) convertMultiMessage(value interface{}) (message.Composer, error) {
	switch data := value.(type) {
	case []byte:
		return lp.convertMultiMessage(bytes.Split(data, []byte("\x00")))
	case string:
		return lp.convertMultiMessage(strings.Split(data, "\n"))
	case []string:
		batch := []message.Composer{}
		for _, str := range data {
			elem, err := lp.produceMessage([]byte(str))
			if err != nil {
				return nil, err
			}
			batch = append(batch, elem)
		}
		return message.MakeGroupComposer(batch), nil
	case []interface{}:
		batch := []message.Composer{}
		for _, dt := range data {
			elem, err := lp.convertMessage(dt)
			if err != nil {
				return nil, err
			}
			batch = append(batch, elem)
		}
		return message.MakeGroupComposer(batch), nil
	default:
		return message.Convert(value), nil
	}
}

func (lp *LoggingPayload) convertMessage(value interface{}) (message.Composer, error) {
	switch data := value.(type) {
	case []byte:
		return lp.produceMessage(data)
	case string:
		return lp.produceMessage([]byte(data))
	default:
		m := message.Convert(value)
		if lp.AddMetadata {
			m.SetOption(message.OptionIncludeMetadata)
		}
		return m, nil
	}
}

func (lp *LoggingPayload) produceMessage(data []byte) (message.Composer, error) {
	switch lp.Format {
	case LoggingPayloadFormatJSON:
		payload := message.Fields{}
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, fmt.Errorf("problem parsing json from message body: %w", err)
		}

		m := message.MakeFields(payload)
		if lp.AddMetadata {
			m.SetOption(message.OptionIncludeMetadata)
		}

		return m, nil
	case LoggingPayloadFormatSTRING:
		m := message.MakeString(string(data))
		if lp.AddMetadata {
			m.SetOption(message.OptionIncludeMetadata)
		}

		return m, nil
	default:
		m := message.MakeBytes(data)

		if lp.AddMetadata {
			m.SetOption(message.OptionIncludeMetadata)
		}

		return m, nil
	}
}
