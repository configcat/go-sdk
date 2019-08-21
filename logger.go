package configcat

import (
	"fmt"
	"log"
	"os"
)

// Logger defines the interface this library logs with
type Logger interface {
	Prefix(string) Logger

	Print(...interface{})
	Printf(string, ...interface{})
}

type logger struct {
	*log.Logger
}

// DefaultLogger instantiates a default logger backed by the standard library logger
func DefaultLogger(name string) Logger {
	return &logger{log.New(os.Stderr, fmt.Sprintf("[%s]", name), log.LstdFlags)}
}

func (l *logger) Prefix(name string) Logger {
	return DefaultLogger(name)
}
