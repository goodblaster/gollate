package logger

// Logger is an interface for logging that matches the logos API.
// This allows the library to be decoupled from any specific logging implementation.
type Logger interface {
	// Debug logs a debug message
	Debug(a ...any)

	// Debugf logs a formatted debug message
	Debugf(format string, args ...any)

	// Error logs an error message
	Error(a ...any)

	// Errorf logs a formatted error message
	Errorf(format string, args ...any)

	// Fatal logs a fatal message and exits
	Fatal(a ...any)

	// WithError returns a logger with error context
	WithError(err error) Logger
}

// Noop is a no-op logger that discards all log messages.
// Use this when you don't want any logging output.
type Noop struct{}

func (n Noop) Debug(a ...any)                    {}
func (n Noop) Debugf(format string, args ...any) {}
func (n Noop) Error(a ...any)                    {}
func (n Noop) Errorf(format string, args ...any) {}
func (n Noop) Fatal(a ...any)                    {}
func (n Noop) WithError(err error) Logger        { return n }

// Default returns a default logger (currently Noop).
// Library consumers should provide their own logger implementation.
func Default() Logger {
	return Noop{}
}
