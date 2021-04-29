package logging

import (
	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Logger is a simple wrapper around go-logr to simplify distinguishing debug
// logs from info.
type Logger interface {
	logr.Logger
	Debug(msg string, keysAndValues ...interface{})
}

type logger struct {
	logr.Logger
}

func (l *logger) Info(msg string, keysAndValues ...interface{}) {
	l.Logger.Info(msg, keysAndValues...)
}

func (l *logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Logger.V(1).Info(msg, keysAndValues...)
}

// NewLogger creates a new logger using the specified name and keys/values.
func NewLogger(name string, keysAndValues ...interface{}) Logger {
	return &logger{
		Logger: logf.Log.WithName(name).WithValues(keysAndValues...),
	}
}

func WithValues(l logr.Logger, keysAndValues ...interface{}) Logger {
	return &logger{Logger: l.WithValues(keysAndValues...)}
}
