/*
Copyright 2026 Shane Utt.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package utils provides testing utilities for integration and unit tests.
package utils

import (
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
)

// -----------------------------------------------------------------------------
// Test Event Recorder
// -----------------------------------------------------------------------------

type testRecorder struct{}

// NewTestRecorder creates a no-op event recorder for testing
func NewTestRecorder() events.EventRecorder {
	return &testRecorder{}
}

// Eventf records an event.
func (r *testRecorder) Eventf(regarding runtime.Object, related runtime.Object, eventtype, reason, action, note string, args ...any) {
}

// WithLogger returns the recorder with a logger.
func (r *testRecorder) WithLogger(logger klog.Logger) events.EventRecorderLogger {
	return r
}

// -----------------------------------------------------------------------------
// Test Logger
// -----------------------------------------------------------------------------

type testLogger struct {
	t *testing.T
}

// NewTestLogger creates a logr.Logger that logs via testing.T
func NewTestLogger(t *testing.T) logr.Logger {
	return logr.New(&testLogger{t: t})
}

// -----------------------------------------------------------------------------
// Test Logger - LogSink Implementation
// -----------------------------------------------------------------------------

// Init initializes the logger with runtime information
func (l *testLogger) Init(info logr.RuntimeInfo) {}

// Enabled returns whether logging is enabled at the given level
func (l *testLogger) Enabled(level int) bool {
	return true // always true for testing
}

// Info logs informational messages to the test output
func (l *testLogger) Info(level int, msg string, keysAndValues ...any) {
	l.t.Logf("[INFO] %s %v", msg, keysAndValues)
}

// Error logs error messages to the test output
func (l *testLogger) Error(err error, msg string, keysAndValues ...any) {
	l.t.Logf("[ERROR] %s: %v %v", msg, err, keysAndValues)
}

// WithValues returns the logger with additional key-value pairs
func (l *testLogger) WithValues(keysAndValues ...any) logr.LogSink {
	return l
}

// WithName returns the logger with an additional name segment
func (l *testLogger) WithName(name string) logr.LogSink {
	return l
}
