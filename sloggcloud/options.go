package sloggcloud

import (
	"log/slog"
)

// options contains configuration options for the Handler.
type options struct {
	level        slog.Level
	addSource    bool
	addTraceInfo bool
	projectID    string
	program      string
}

// Option is a function that configures the Handler.
type Option func(*options)

// defaultOptions returns the default options.
func defaultOptions() *options {
	return &options{
		level:        slog.LevelInfo,
		addSource:    false,
		addTraceInfo: true,
		projectID:    "",
		program:      "",
	}
}

// WithLevel sets the minimum level to log.
func WithLevel(level slog.Level) Option {
	return func(o *options) {
		o.level = level
	}
}

// WithSource enables source code location in logs.
func WithSource(enabled bool) Option {
	return func(o *options) {
		o.addSource = enabled
	}
}

// WithTraceInfo enables adding trace and span IDs to logs.
func WithTraceInfo(enabled bool) Option {
	return func(o *options) {
		o.addTraceInfo = enabled
	}
}

// WithProjectID sets the Google Cloud project ID for trace formatting.
func WithProjectID(projectID string) Option {
	return func(o *options) {
		o.projectID = projectID
	}
}

// WithProgram sets the program name.
func WithProgram(program string) Option {
	return func(o *options) {
		o.program = program
	}
}
