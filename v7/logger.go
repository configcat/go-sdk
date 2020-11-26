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
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})

	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})

	Debugln(args ...interface{})
	Infoln(args ...interface{})
	Warnln(args ...interface{})
	Errorln(args ...interface{})
}

// LoggerWithLevel is optionally implemented by a Logger.
// It is notably implemented by logrus.Logger and thus
// by the DefaultLogger returned by this package.
type LoggerWithLevel interface {
	// GetLevel returns the current logging level.
	GetLevel() LogLevel
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
