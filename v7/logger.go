package configcat

import (
	"github.com/sirupsen/logrus"
)

// Define the logrus log levels
const (
	LogLevelPanic = logrus.PanicLevel
	LogLevelFatal = logrus.FatalLevel
	LogLevelError = logrus.ErrorLevel
	LogLevelWarn  = logrus.WarnLevel
	LogLevelInfo  = logrus.InfoLevel
	LogLevelDebug = logrus.DebugLevel
	LogLevelTrace = logrus.TraceLevel
)

type LogLevel = logrus.Level

// Logger defines the interface this library logs with.
type Logger interface {
	// GetLevel returns the current logging level.
	GetLevel() LogLevel

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// DefaultLogger creates the default logger with specified log level (logrus.New()).
func DefaultLogger(level LogLevel) Logger {
	logger := logrus.New()
	logger.SetLevel(level)
	return logger
}

// leveledLogger wraps a Logger for efficiency reasons: it's a static type
// rather than an interface so the compiler can inline the level check
// and thus avoid the allocation for the arguments.
type leveledLogger struct {
	level LogLevel
	Logger
}

func (log *leveledLogger) enabled(level LogLevel) bool {
	return level <= log.level
}
