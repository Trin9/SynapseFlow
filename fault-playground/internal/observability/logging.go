package observability

import (
	"context"
	"fmt"
	"log"
	"os"
)

// Logger abstracts structured logging for the playground services.
type Logger interface {
	Info(ctx context.Context, msg string, fields map[string]interface{})
	Error(ctx context.Context, msg string, fields map[string]interface{})
}

type defaultLogger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
}

// NewLogger returns a logger implementation.
func NewLogger() Logger {
	return &defaultLogger{
		infoLogger:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func (l *defaultLogger) Info(ctx context.Context, msg string, fields map[string]interface{}) {
	l.infoLogger.Output(2, formatLogMessage(msg, fields))
}

func (l *defaultLogger) Error(ctx context.Context, msg string, fields map[string]interface{}) {
	l.errorLogger.Output(2, formatLogMessage(msg, fields))
}

func formatLogMessage(msg string, fields map[string]interface{}) string {
	logMsg := msg
	if len(fields) > 0 {
		logMsg += " {"
		first := true
		for k, v := range fields {
			if !first {
				logMsg += ", "
			}
			logMsg += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		logMsg += "}"
	}
	return logMsg
}
