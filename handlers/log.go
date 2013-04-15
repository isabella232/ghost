package handlers

// Inspired by node's Connect library implementation of the logging middleware
// https://github.com/senchalabs/connect

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"
)

const (
	// Predefined logging formats that can be passed as format string.
	Ldefault = "_default_"
	Lshort   = "_short_"
	Ltiny    = "_tiny_"
)

var (
	// Token parser for request and response headers
	rxHeaders = regexp.MustCompile(`^(req|res)\[([^\]]+)\]$`)

	// Lookup table for predefined formats
	predefFormats = map[string]struct {
		fmt  string
		toks []string
	}{
		Ldefault: {
			`%s - - [%s] "%s %s HTTP/%s" %d %s "%s" "%s"`,
			[]string{"remote-addr", "date", "method", "url", "http-version", "status", "res[Content-Length]", "referrer", "user-agent"},
		},
		Lshort: {
			`%s - %s %s HTTP/%s %d %s - %.3f s`,
			[]string{"remote-addr", "method", "url", "http-version", "status", "res[Content-Length]", "response-time"},
		},
		Ltiny: {
			`%s %s %d %s - %.3f s`,
			[]string{"method", "url", "status", "res[Content-Length]", "response-time"},
		},
	}
)

// Augmented ResponseWriter implementation that captures the status code for the logger.
type statusResponseWriter struct {
	http.ResponseWriter
	code int
}

// Intercept the WriteHeader call to save the status code.
func (this *statusResponseWriter) WriteHeader(code int) {
	this.code = code
	this.ResponseWriter.WriteHeader(code)
}

// Intercept the Write call to save the default status code.
func (this *statusResponseWriter) Write(data []byte) (int, error) {
	if this.code == 0 {
		this.code = http.StatusOK
	}
	return this.ResponseWriter.Write(data)
}

// Implement the WrapWriter interface.
func (this *statusResponseWriter) WrappedWriter() http.ResponseWriter {
	return this.ResponseWriter
}

// LogHandler options
type LogOptions struct {
	Logger       *log.Logger
	Format       string
	Tokens       []string
	CustomTokens map[string]func(http.ResponseWriter, *http.Request) string
	Immediate    bool
	DateFormat   string
}

// Create a new LogOptions struct. The DateFormat defaults to time.RFC3339.
func NewLogOptions(l *log.Logger, ft string, tok ...string) *LogOptions {
	return &LogOptions{
		Logger:       l,
		Format:       ft,
		Tokens:       tok,
		CustomTokens: make(map[string]func(http.ResponseWriter, *http.Request) string),
		DateFormat:   time.RFC3339,
	}
}

// Create a log handler for every request it receives.
func LogHandler(h http.Handler, opts *LogOptions) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if _, ok := getStatusWriter(w); ok {
				// Self-awareness, logging handler already set up
				h.ServeHTTP(w, r)
				return
			}

			// Save the response start time
			st := time.Now()
			// Call the wrapped handler, with the augmented ResponseWriter to handle the status code
			stw := &statusResponseWriter{w, 0}

			// Log immediately if requested, otherwise on exit
			if opts.Immediate {
				logRequest(stw, r, st, opts)
			} else {
				defer logRequest(stw, r, st, opts)
			}
			h.ServeHTTP(stw, r)
		})
}

// Check if the specified token is a predefined one, and if so return its current value.
func getPredefinedTokenValue(t string, w *statusResponseWriter, r *http.Request,
	st time.Time, opts *LogOptions) (interface{}, bool) {

	switch t {
	case "http-version":
		return fmt.Sprintf("%d.%d", r.ProtoMajor, r.ProtoMinor), true
	case "response-time":
		return time.Now().Sub(st).Seconds(), true
	case "remote-addr":
		return r.RemoteAddr, true
	case "date":
		return time.Now().Format(opts.DateFormat), true
	case "method":
		return r.Method, true
	case "url":
		return r.URL.String(), true
	case "referrer", "referer":
		return r.Referer(), true
	case "user-agent":
		return r.UserAgent(), true
	case "status":
		return w.code, true
	}

	// Handle special cases for header
	mtch := rxHeaders.FindStringSubmatch(t)
	if len(mtch) > 2 {
		if mtch[1] == "req" {
			return r.Header.Get(mtch[2]), true
		} else {
			// TODO : This only works for headers explicitly set via the Header() map of
			// the writer, not those added by the http package under the covers.
			return w.Header().Get(mtch[2]), true
		}
	}
	return nil, false
}

// Do the actual logging.
func logRequest(w *statusResponseWriter, r *http.Request, st time.Time, opts *LogOptions) {
	var (
		fn     func(string, ...interface{})
		ok     bool
		format string
		toks   []string
	)

	// If no specific logger, use the default one from the log package
	if opts.Logger == nil {
		fn = log.Printf
	} else {
		fn = opts.Logger.Printf
	}

	// If this is a predefined format, use it instead
	if v, ok := predefFormats[opts.Format]; ok {
		format = v.fmt
		toks = v.toks
	} else {
		format = opts.Format
		toks = opts.Tokens
	}
	args := make([]interface{}, len(toks))
	for i, t := range toks {
		if args[i], ok = getPredefinedTokenValue(t, w, r, st, opts); !ok {
			if f, ok := opts.CustomTokens[t]; ok && f != nil {
				args[i] = f(w, r)
			} else {
				args[i] = "?"
			}
		}
	}
	fn(format, args...)
}

// Helper function to retrieve the status writer.
func getStatusWriter(w http.ResponseWriter) (*statusResponseWriter, bool) {
	st, ok := GetResponseWriter(w, func(tst http.ResponseWriter) bool {
		_, ok := tst.(*statusResponseWriter)
		return ok
	})
	if ok {
		return st.(*statusResponseWriter), true
	}
	return nil, false
}