package httpmock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// Server simplifies the orchestration of a [Mock] inside a handler and server.
// It wraps the stdlib [httptest.Server] implementation and provides a
// handler to log requests and write configured responses.
type Server struct {
	*httptest.Server

	Mock *Mock

	// Whether or not panics should be caught in the server goroutine or
	// allowed to propagate to the parent process. If false, the panic will be
	// printed and a 404 will be returned to the client.
	ignorePanic bool
}

// ServerConfig contains settings for configuring a [Server]. It is used with
// [NewServerWithConfig]. For default behavior, use [NewServer].
type ServerConfig struct {
	// Create TLS-configured server
	TLS bool

	// Custom server handler
	Handler http.HandlerFunc
}

// makeHandler creates a standard [http.HandlerFunc] that may be used by a
// regular or TLS [Server] to log requests and write configured responses.
func makeHandler(s *Server) http.HandlerFunc {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rc := recover(); rc != nil {
					if s.IsRecoverable() {
						fmt.Printf("%v\n", rc)

						w.WriteHeader(http.StatusNotFound)
					} else {
						panic(rc)
					}

				}
			}()

			response := s.Mock.Requested(r)
			if _, err := response.Write(w, r); err != nil {
				s.Mock.fail("failed to write response for request:\n%s\nwith error: %v", response.parent.String(), err)
			}
		},
	)
}

// NewServer creates a new [Server] and associated [Mock].
func NewServer() *Server {
	s := &Server{Mock: new(Mock)}
	s.Server = httptest.NewServer(http.HandlerFunc(makeHandler(s)))

	return s
}

func NewServerWithConfig(cfg ServerConfig) *Server {
	s := &Server{Mock: new(Mock)}

	handler := cfg.Handler
	if handler == nil {
		handler = http.HandlerFunc(makeHandler(s))
	}

	if cfg.TLS {
		s.Server = httptest.NewTLSServer(handler)
	} else {
		s.Server = httptest.NewServer(handler)
	}

	return s
}

// NotRecoverable sets a [Server] as not recoverable, so that panics are allowed
// to propagate to the main process. With the default handler, panics are caught
// and printed to stdout, with a final 404 returned to the client.
//
// 404 was chosen rather than 500 due to panics almost always occurring when a
// matching [Request] cannot be found. However, custom handlers can choose to
// implement their recovery mechanism however they would like, using the
// [Server.IsRecoverable] method to access this value.
func (s *Server) NotRecoverable() *Server {
	s.ignorePanic = true
	return s
}

// IsRecoverable returns whether or not the [Server] is considered recoverable.
func (s *Server) IsRecoverable() bool {
	return !s.ignorePanic
}

// On is a convenience method to invoke the [Mock.On] method.
//
//	Server.On(http.MethodDelete, "/some/path/1234")
func (s *Server) On(method string, URL string, body []byte) *Request {
	return s.Mock.On(method, URL, body)
}
