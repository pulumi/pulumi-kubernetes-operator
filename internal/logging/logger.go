// Copyright 2021, Pulumi Corporation.  All rights reserved.

package logging

import (
	"bufio"
	"io"

	"github.com/go-logr/logr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Logger struct {
	logr.Logger
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.Logger.Info(msg, keysAndValues...)
}

func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Logger.V(1).Info(msg, keysAndValues...)
}

func (l *Logger) LogWriterDebug(msg string, keysAndValues ...interface{}) io.WriteCloser {
	return l.logWriter(l.Debug, msg, keysAndValues...)
}

func (l *Logger) LogWriterInfo(msg string, keysAndValues ...interface{}) io.WriteCloser {
	return l.logWriter(l.Info, msg, keysAndValues...)
}

// logWriter constructs an io.Writer that logs to the provided logging.Logger
func (l *Logger) logWriter(logFunc func(msg string, keysAndValues ...interface{}),
	msg string,
	keysAndValues ...interface{}) io.WriteCloser {

	stdoutR, stdoutW := io.Pipe()
	go func() {
		defer contract.IgnoreClose(stdoutR)
		outs := bufio.NewScanner(stdoutR)
		for outs.Scan() {
			text := outs.Text()
			logFunc(msg, append([]interface{}{"Stdout", text}, keysAndValues...)...)
		}
		err := outs.Err()
		if err != nil {
			l.Error(err, msg, keysAndValues...)
		}
	}()
	return stdoutW
}

// NewLogger creates a new Logger using the specified name and keys/values.
func NewLogger(name string, keysAndValues ...interface{}) *Logger {
	return &Logger{
		Logger: logf.Log.WithName(name).WithValues(keysAndValues...),
	}
}

// WithValues creates a new Logger using the passed logr.Logger with
// the specified key/values.
func WithValues(l logr.Logger, keysAndValues ...interface{}) *Logger {
	return &Logger{Logger: l.WithValues(keysAndValues...)}
}
