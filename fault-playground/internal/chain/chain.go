package chain

import "context"

// ServiceName identifies a service participating in the demo chain.
type ServiceName string

const (
	ServiceA ServiceName = "svc-a"
	ServiceB ServiceName = "svc-b"
	ServiceC ServiceName = "svc-c"
	ServiceD ServiceName = "svc-d"
)

// Request carries the minimal fields passed across the chain.
type Request struct {
	TraceID string
	UserID  string
	Profile *Profile
}

// Profile represents a nested payload that can be dropped or corrupted.
type Profile struct {
	ID    string
	Email string
}

// Response represents the service response used in the demo chain.
type Response struct {
	TraceID string
	Message string
}

// Chain defines the request routing across services A/B/C/D.
type Chain interface {
	CallA(ctx context.Context, req Request) (Response, error)
	CallB(ctx context.Context, req Request) (Response, error)
	CallC(ctx context.Context, req Request) (Response, error)
	CallD(ctx context.Context, req Request) (Response, error)
}
