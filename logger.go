package configcat

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

const (
	LogLevelDebug = -2
	LogLevelInfo  = -1
	LogLevelWarn  = 0
	LogLevelError = 1
	LogLevelNone  = 2
)

type LogLevel int

// Logger defines the interface this library logs with.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// DefaultLogger creates the default logger with specified log level.
func DefaultLogger() Logger {
	return &defaultLogger{Logger: log.New(os.Stderr, "[ConfigCat] ", log.LstdFlags)}
}

func newLeveledLogger(logger Logger, level LogLevel, hooks *Hooks) *leveledLogger {
	if logger == nil {
		logger = DefaultLogger()
	}
	return &leveledLogger{
		minLevel: level,
		Logger:   logger,
		hooks:    hooks,
	}
}

// leveledLogger wraps a Logger for efficiency reasons: it's a static type
// rather than an interface so the compiler can inline the level check
// and thus avoid the allocation for the arguments.
type leveledLogger struct {
	minLevel LogLevel
	hooks    *Hooks
	Logger
}

type defaultLogger struct {
	*log.Logger
}

func (log *leveledLogger) enabled(level LogLevel) bool {
	return level >= log.minLevel
}

func (log *leveledLogger) Debugf(format string, args ...interface{}) {
	if log.enabled(LogLevelDebug) {
		log.Logger.Debugf("[0] "+format, args...)
	}
}

func (log *leveledLogger) Infof(eventId int, format string, args ...interface{}) {
	if log.enabled(LogLevelInfo) {
		log.Logger.Infof("["+strconv.Itoa(eventId)+"] "+format, args...)
	}
}

func (log *leveledLogger) Warnf(eventId int, format string, args ...interface{}) {
	if log.enabled(LogLevelWarn) {
		log.Logger.Warnf("["+strconv.Itoa(eventId)+"] "+format, args...)
	}
}

func (log *leveledLogger) Errorf(eventId int, format string, args ...interface{}) {
	if log.hooks != nil && log.hooks.OnError != nil {
		go log.hooks.OnError(fmt.Errorf(format, args...))
	}
	if log.enabled(LogLevelError) {
		log.Logger.Errorf("["+strconv.Itoa(eventId)+"] "+format, args...)
	}
}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	l.logf(LogLevelDebug, format, args...)
}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	l.logf(LogLevelInfo, format, args...)
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	l.logf(LogLevelWarn, format, args...)
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	l.logf(LogLevelError, format, args...)
}

func (l *defaultLogger) logf(level LogLevel, format string, args ...interface{}) {
	l.Logger.Printf(level.String()+": "+format, args...)
}

func (lvl LogLevel) String() string {
	switch lvl {
	case LogLevelDebug:
		return "[DEBUG]"
	case LogLevelInfo:
		return "[INFO]"
	case LogLevelWarn:
		return "[WARN]"
	case LogLevelError:
		return "[ERROR]"
	}
	return "[UNKNOWN]"
}
