package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// HTTPError is an error with a status code and a message safe to show clients.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }

func httpErr(status int, format string, args ...any) error {
	return &HTTPError{Status: status, Message: fmt.Sprintf(format, args...)}
}

// handlerFunc is an http handler that reports failures by returning an error.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

// handle turns a handlerFunc into an http.HandlerFunc that writes errors as JSON.
func (s *Server) handle(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err == nil {
			return
		}
		var he *HTTPError
		if errors.As(err, &he) {
			respond(w, he.Status, map[string]string{"error": he.Message})
			return
		}
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
	}
}

// respond writes v as JSON with the given status; a nil v writes no body.
func respond(w http.ResponseWriter, status int, v any) error {
	if v == nil {
		w.WriteHeader(status)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
	return nil
}

// decodeJSON reads the request body into dst. An empty body leaves dst unchanged.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return httpErr(http.StatusBadRequest, "Invalid JSON body")
	}
	return nil
}

// pathID parses a numeric path parameter; invalid values become 0, which matches no row.
func pathID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id
}

func round2(x float64) float64 {
	return math.Round(x*100) / 100
}

// cors lets browsers on the configured origins call the API. Requests from
// other origins get no CORS headers, so the browser blocks them.
func (s *Server) cors(next http.Handler) http.Handler {
	allowAny := slices.Contains(s.cfg.CORSOrigins, "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin != "" && (allowAny || slices.Contains(s.cfg.CORSOrigins, origin)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}
