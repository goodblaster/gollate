package logger

import (
	"io"

	"github.com/goodblaster/logos"
)

// LogosAdapter wraps the logos package to implement our Logger interface.
type LogosAdapter struct {
	err error
}

// NewLogos creates a new logger that uses the logos package.
// It writes to logos's default destination (stdout). For CLI tools that
// emit data on stdout, use NewLogosTo(os.Stderr) so logs never corrupt
// piped output.
func NewLogos() Logger {
	return &LogosAdapter{}
}

// NewLogosTo creates a logos-backed logger that writes to w.
func NewLogosTo(w io.Writer) Logger {
	return &logosInstance{log: logos.NewLogger(logos.LevelDebug, logos.ConsoleFormatter(), w)}
}

// logosInstance adapts a dedicated logos.Logger instance (with its own
// writer) instead of the package-level default logger.
type logosInstance struct {
	log logos.Logger
}

func (l *logosInstance) Debug(a ...any)                    { l.log.Debug(a...) }
func (l *logosInstance) Debugf(format string, args ...any) { l.log.Debugf(format, args...) }
func (l *logosInstance) Error(a ...any)                    { l.log.Error(a...) }
func (l *logosInstance) Errorf(format string, args ...any) { l.log.Errorf(format, args...) }
func (l *logosInstance) Fatal(a ...any)                    { l.log.Fatal(a...) }
func (l *logosInstance) WithError(err error) Logger {
	return &logosInstance{log: l.log.WithError(err)}
}

func (l *LogosAdapter) Debug(a ...any) {
	if l.err != nil {
		logos.WithError(l.err).Debug(a...)
	} else {
		logos.Debug(a...)
	}
}

func (l *LogosAdapter) Debugf(format string, args ...any) {
	if l.err != nil {
		logos.WithError(l.err).Debugf(format, args...)
	} else {
		logos.Debugf(format, args...)
	}
}

func (l *LogosAdapter) Error(a ...any) {
	if l.err != nil {
		logos.WithError(l.err).Error(a...)
	} else {
		logos.Error(a...)
	}
}

func (l *LogosAdapter) Errorf(format string, args ...any) {
	if l.err != nil {
		logos.WithError(l.err).Errorf(format, args...)
	} else {
		logos.Errorf(format, args...)
	}
}

func (l *LogosAdapter) Fatal(a ...any) {
	if l.err != nil {
		logos.WithError(l.err).Fatal(a...)
	} else {
		logos.Fatal(a...)
	}
}

func (l *LogosAdapter) WithError(err error) Logger {
	return &LogosAdapter{err: err}
}
