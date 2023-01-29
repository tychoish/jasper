package options

import (
	"encoding/json"
	"sync"

	"github.com/tychoish/birch"
	"github.com/tychoish/grip/send"
)

var globalLoggerRegistry LoggerRegistry

func init() {
	globalLoggerRegistry = &basicLoggerRegistry{
		factories: map[string]LoggerProducerFactory{
			LogDefault:   NewDefaultLoggerProducer,
			LogFile:      NewFileLoggerProducer,
			LogInherited: NewInheritedLoggerProducer,
			LogInMemory:  NewInMemoryLoggerProducer,
			LogSplunk:    NewSplunkLoggerProducer,
		},
		marshalers: map[RawLoggerConfigFormat]Marshaler{
			RawLoggerConfigFormatJSON: json.Marshal,
			RawLoggerConfigFormatBSON: func(val interface{}) ([]byte, error) {
				return birch.DC.Interface(val).MarshalBSON()
			},
		},
		unmarshalers: map[RawLoggerConfigFormat]Unmarshaler{
			RawLoggerConfigFormatJSON: json.Unmarshal,
			RawLoggerConfigFormatBSON: func(data []byte, val interface{}) error {
				doc, err := birch.DCE.Reader(data)
				if err != nil {
					return err
				}

				return doc.Unmarshal(val)
			},
		},
	}
}

// GetGlobalLoggerRegistry returns the global logger registry.
func GetGlobalLoggerRegistry() LoggerRegistry { return globalLoggerRegistry }

// LoggerRegistry is an interface that stores reusable logger factories.
type LoggerRegistry interface {
	Register(LoggerProducerFactory)
	Check(string) bool
	Names() []string
	Resolve(string) (LoggerProducerFactory, bool)

	RegisterUnmarshaler(RawLoggerConfigFormat, Unmarshaler)
	RegisterMarshaler(RawLoggerConfigFormat, Marshaler)
	Unmarshaler(RawLoggerConfigFormat) Unmarshaler
	Marshaler(RawLoggerConfigFormat) Marshaler
}

type Marshaler func(interface{}) ([]byte, error)
type Unmarshaler func([]byte, interface{}) error

type basicLoggerRegistry struct {
	mu           sync.RWMutex
	factories    map[string]LoggerProducerFactory
	marshalers   map[RawLoggerConfigFormat]Marshaler
	unmarshalers map[RawLoggerConfigFormat]Unmarshaler
}

// NewBasicLoggerRegsitry returns a new LoggerRegistry backed by the
// basicLoggerRegistry implementation.
func NewBasicLoggerRegistry() LoggerRegistry {
	return &basicLoggerRegistry{
		factories:    map[string]LoggerProducerFactory{},
		marshalers:   map[RawLoggerConfigFormat]Marshaler{},
		unmarshalers: map[RawLoggerConfigFormat]Unmarshaler{},
	}
}

func (r *basicLoggerRegistry) RegisterUnmarshaler(f RawLoggerConfigFormat, um Unmarshaler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.unmarshalers[f] = um
}
func (r *basicLoggerRegistry) RegisterMarshaler(f RawLoggerConfigFormat, m Marshaler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.marshalers[f] = m
}

func (r *basicLoggerRegistry) Unmarshaler(f RawLoggerConfigFormat) Unmarshaler {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.unmarshalers[f]
}

func (r *basicLoggerRegistry) Marshaler(f RawLoggerConfigFormat) Marshaler {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.marshalers[f]
}

func (r *basicLoggerRegistry) Register(factory LoggerProducerFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[factory().Type()] = factory
}

func (r *basicLoggerRegistry) Check(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.factories[name]
	return ok
}

func (r *basicLoggerRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := []string{}
	for name := range r.factories {
		names = append(names, name)
	}

	return names
}

func (r *basicLoggerRegistry) Resolve(name string) (LoggerProducerFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.factories[name]
	return factory, ok
}

// LoggerProducer produces a Logger interface backed by a grip logger.
type LoggerProducer interface {
	Type() string
	Configure() (send.Sender, error)
}

// LoggerProducerFactory creates a new instance of a LoggerProducer implementation.
type LoggerProducerFactory func() LoggerProducer
