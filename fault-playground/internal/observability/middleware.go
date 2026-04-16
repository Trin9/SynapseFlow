package observability

import (
	"fmt"
	"net/http"

	// We need to import "runtime" to get the call stack.
	"runtime"
)

// PanicRecoveryMiddleware recovers from panics and logs them.
func PanicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rcv := recover(); rcv != nil {
				// Get the logger instance.
				// This assumes that the logger is initialized and accessible.
				// In a real application, you might want to inject the logger.
				logger := NewLogger() // Placeholder - ideally injected.

				// Get stack trace.
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)

			fields := map[string]interface{}{
				"error":   rcv,
				"stack":   string(buf[:n]),
				"request": r.URL.String(),
			}
			logger.Error(r.Context(), "Panic recovered", fields)

			// Optionally, you can write a response to the client.
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}()

		next.ServeHTTP(w, r)
	})
}
