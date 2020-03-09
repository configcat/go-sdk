package configcat

import (
	"github.com/sirupsen/logrus"
)

// Define the logrus log levels
const (
	LogLevelPanic = LogLevel(logrus.PanicLevel)
	LogLevelFatal = LogLevel(logrus.FatalLevel)
	LogLevelError = LogLevel(logrus.ErrorLevel)
	LogLevelWarn  = LogLevel(logrus.WarnLevel)
	LogLevelInfo  = LogLevel(logrus.InfoLevel)
	LogLevelDebug = LogLevel(logrus.DebugLevel)
	LogLevelTrace = LogLevel(logrus.TraceLevel)
)

type LogLevel uint32

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

// DefaultLogger creates the default logger with specified log level (logrus.New()).
func DefaultLogger(level LogLevel) Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.Level(level))
	return logger
}
