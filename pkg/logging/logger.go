// Copyright 2021, Pulumi Corporation.  All rights reserved.

package logging

import (
	"bufio"
	"github.com/go-logr/logr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"io"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Logger is a simple wrapper around go-logr to simplify distinguishing debug
// logs from info.
type Logger interface {
	logr.Logger

	// Debug prints the message and key/values at debug level (level 1) in go-logr
	Debug(msg string, keysAndValues ...interface{})

	// LogWriterDebug returns the write end of a pipe which streams data to
	// the logger at debug level.
	LogWriterDebug(msg string, keysAndValues ...interface{}) io.WriteCloser

	// LogWriterInfo returns the write end of a pipe which streams data to
	// the logger at info level.
	LogWriterInfo(msg string, keysAndValues ...interface{}) io.WriteCloser
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

func (l *logger) LogWriterDebug(msg string, keysAndValues ...interface{}) io.WriteCloser {
	return l.logWriter(l.Debug, msg, keysAndValues...)
}

func (l *logger) LogWriterInfo(msg string, keysAndValues ...interface{}) io.WriteCloser {
	return l.logWriter(l.Info, msg, keysAndValues...)
}

// logWriter constructs an io.Writer that logs to the provided logging.Logger
func (l *logger) logWriter(logFunc func(msg string, keysAndValues ...interface{}),
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
func NewLogger(name string, keysAndValues ...interface{}) Logger {
	return &logger{
		Logger: logf.Log.WithName(name).WithValues(keysAndValues...),
	}
}

// WithValues creates a new Logger using the passed logr.Logger with
// the specified key/values.
func WithValues(l logr.Logger, keysAndValues ...interface{}) Logger {
	return &logger{Logger: l.WithValues(keysAndValues...)}
}
