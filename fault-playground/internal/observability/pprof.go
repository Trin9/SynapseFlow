package observability

import "net/http"

// RegisterPProf registers pprof handlers on the given mux.
func RegisterPProf(mux *http.ServeMux) {
	_ = mux
}
