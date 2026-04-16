package observability

import "context"

// Logger abstracts structured logging for the playground services.
type Logger interface {
	Info(ctx context.Context, msg string, fields map[string]interface{})
	Error(ctx context.Context, msg string, fields map[string]interface{})
}

// NewLogger returns a logger implementation.
func NewLogger() Logger {
	return nil
}
