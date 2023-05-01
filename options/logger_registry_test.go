package options

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tychoish/fun/assert/check"
)

func TestLoggerRegistry(t *testing.T) {
	registry := NewBasicLoggerRegistry()
	registeredFactories := map[string]LoggerProducerFactory{}

	registeredFactories[LogDefault] = NewDefaultLoggerProducer
	check.True(t, !registry.Check(LogDefault))
	registry.Register(NewDefaultLoggerProducer)
	check.True(t, registry.Check(LogDefault))
	factory, ok := registry.Resolve(LogDefault)
	check.Equal(t, NewDefaultLoggerProducer(), factory())
	check.True(t, ok)

	registeredFactories[LogFile] = NewFileLoggerProducer
	check.True(t, !registry.Check(LogFile))
	registry.Register(NewFileLoggerProducer)
	check.True(t, registry.Check(LogFile))
	factory, ok = registry.Resolve(LogFile)
	check.Equal(t, NewFileLoggerProducer(), factory())
	check.True(t, ok)

	factories := registry.Names()
	require.Equal(t, len(factories), len(registeredFactories))
	for _, factoryName := range factories {
		_, ok := registeredFactories[factoryName]
		require.True(t, ok)
	}
}
