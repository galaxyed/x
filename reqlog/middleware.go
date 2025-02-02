package reqlog

import (
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/negroni"

	"github.com/galaxyed/x/logrusx"
)

type timer interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

type realClock struct{}

func (rc *realClock) Now() time.Time {
	return time.Now()
}

func (rc *realClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// Middleware is a middleware handler that logs the request as it goes in and the response as it goes out.
type Middleware struct {
	// Logger is the log.Logger instance used to log messages with the Logger middleware
	Logger *logrusx.Logger
	// Name is the name of the application as recorded in latency metrics
	Name   string
	Before func(*logrusx.Logger, *http.Request, string) *logrusx.Logger
	After  func(*logrusx.Logger, *http.Request, negroni.ResponseWriter, time.Duration, string) *logrusx.Logger

	logStarting bool

	clock timer

	logLevel logrus.Level

	// Silence log for specific URL paths
	silencePaths map[string]bool

	sync.RWMutex
}

// NewMiddleware returns a new *Middleware, yay!
func NewMiddleware() *Middleware {
	return NewCustomMiddleware(logrus.InfoLevel, &logrus.TextFormatter{}, "web")
}

// NewCustomMiddleware builds a *Middleware with the given level and formatter
func NewCustomMiddleware(level logrus.Level, formatter logrus.Formatter, name string) *Middleware {
	log := logrusx.New(name, "", logrusx.ForceFormatter(formatter), logrusx.ForceLevel(level))
	return &Middleware{
		Logger: log,
		Name:   name,
		Before: DefaultBefore,
		After:  DefaultAfter,

		logLevel:     logrus.InfoLevel,
		logStarting:  true,
		clock:        &realClock{},
		silencePaths: map[string]bool{},
	}
}

// NewMiddlewareFromLogger returns a new *Middleware which writes to a given logrus logger.
func NewMiddlewareFromLogger(logger *logrusx.Logger, name string) *Middleware {
	return &Middleware{
		Logger: logger,
		Name:   name,
		Before: DefaultBefore,
		After:  DefaultAfter,

		logLevel:     logrus.InfoLevel,
		logStarting:  true,
		clock:        &realClock{},
		silencePaths: map[string]bool{},
	}
}

// SetLogStarting accepts a bool to control the logging of "started handling
// request" prior to passing to the next middleware
func (m *Middleware) SetLogStarting(v bool) {
	m.logStarting = v
}

// ExcludePaths adds new URL paths to be ignored during logging. The URL u is parsed, hence the returned error
func (m *Middleware) ExcludePaths(paths ...string) *Middleware {
	for _, path := range paths {
		m.Lock()
		m.silencePaths[path] = true
		m.Unlock()
	}
	return m
}

func (m *Middleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if m.Before == nil {
		m.Before = DefaultBefore
	}

	if m.After == nil {
		m.After = DefaultAfter
	}

	logLevel := m.logLevel
	m.RLock()
	if _, ok := m.silencePaths[r.URL.Path]; ok {
		logLevel = logrus.TraceLevel
	}
	m.RUnlock()

	start := m.clock.Now()

	// Try to get the real IP
	remoteAddr := r.RemoteAddr
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		remoteAddr = realIP
	}

	entry := m.Logger.NewEntry()

	entry = m.Before(entry, r, remoteAddr)

	if m.logStarting {
		entry.Log(logLevel, "started handling request")
	}

	next(rw, r)

	latency := m.clock.Since(start)
	res := rw.(negroni.ResponseWriter)

	m.After(entry, r, res, latency, m.Name).Log(logLevel, "completed handling request")
}

// BeforeFunc is the func type used to modify or replace the *logrusx.Logger prior
// to calling the next func in the middleware chain
type BeforeFunc func(*logrusx.Logger, *http.Request, string) *logrusx.Logger

// AfterFunc is the func type used to modify or replace the *logrusx.Logger after
// calling the next func in the middleware chain
type AfterFunc func(*logrusx.Logger, negroni.ResponseWriter, time.Duration, string) *logrusx.Logger

// DefaultBefore is the default func assigned to *Middleware.Before
func DefaultBefore(entry *logrusx.Logger, req *http.Request, remoteAddr string) *logrusx.Logger {
	return entry.WithRequest(req)
}

// DefaultAfter is the default func assigned to *Middleware.After
func DefaultAfter(entry *logrusx.Logger, req *http.Request, res negroni.ResponseWriter, latency time.Duration, name string) *logrusx.Logger {
	return entry.WithRequest(req).WithField("http_response", map[string]interface{}{
		"status":      res.Status(),
		"size":        res.Size(),
		"text_status": http.StatusText(res.Status()),
		"took":        latency,
		"headers":     entry.HTTPHeadersRedacted(res.Header()),
	})
}
