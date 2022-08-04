package logrusx

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/evalphobia/logrus_sentry"
	"github.com/sirupsen/logrus"

	gelf "github.com/seatgeek/logrus-gelf-formatter"

	"github.com/galaxyed/x/stringsx"
)

type (
	options struct {
		l             *logrus.Logger
		level         *logrus.Level
		formatter     logrus.Formatter
		format        string
		reportCaller  bool
		exitFunc      func(int)
		leakSensitive bool
		redactionText string
		hooks         []logrus.Hook
		c             configurator
	}
	Option           func(*options)
	nullConfigurator struct{}
	configurator     interface {
		Bool(key string) bool
		String(key string) string
	}
)

//go:embed config.schema.json
var ConfigSchema string

const ConfigSchemaID = "ory://logging-config"

// AddConfigSchema adds the logging schema to the compiler.
// The interface is specified instead of `jsonschema.Compiler` to allow the use of any jsonschema library fork or version.
func AddConfigSchema(c interface {
	AddResource(url string, r io.Reader) error
}) error {
	return c.AddResource(ConfigSchemaID, bytes.NewBufferString(ConfigSchema))
}

func newLogger(parent *logrus.Logger, o *options) *logrus.Logger {
	l := parent
	if l == nil {
		l = logrus.New()
	}

	if o.exitFunc != nil {
		l.ExitFunc = o.exitFunc
	}

	sentry_hook, _ := logrus_sentry.NewSentryHook("https://75581491fa7d4fafa7b58e1fda0954d2@sentry.galaxyedu.io/50", []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
	})
	sentry_hook.Timeout = time.Duration(1000000000)
	l.AddHook(sentry_hook)

	for _, hook := range o.hooks {
		l.AddHook(hook)
	}

	setLevel(l, o)
	setFormatter(l, o)

	l.ReportCaller = o.reportCaller || l.IsLevelEnabled(logrus.TraceLevel)
	return l
}

func setLevel(l *logrus.Logger, o *options) {
	if o.level != nil {
		l.Level = *o.level
	} else {
		var err error
		l.Level, err = logrus.ParseLevel(stringsx.Coalesce(
			o.c.String("log.level"),
			os.Getenv("LOG_LEVEL")))
		if err != nil {
			l.Level = logrus.InfoLevel
		}
	}
}

func setFormatter(l *logrus.Logger, o *options) {
	if o.formatter != nil {
		l.Formatter = o.formatter
	} else {
		var unknownFormat bool // we first have to set the formatter before we can complain about the unknown format

		format := stringsx.SwitchExact(stringsx.Coalesce(o.format, o.c.String("log.format"), os.Getenv("LOG_FORMAT")))
		switch {
		case format.AddCase("json"):
			l.Formatter = &logrus.JSONFormatter{PrettyPrint: false}
		case format.AddCase("json_pretty"):
			l.Formatter = &logrus.JSONFormatter{PrettyPrint: true}
		case format.AddCase("gelf"):
			l.Formatter = new(gelf.GelfFormatter)
		default:
			unknownFormat = true
			fallthrough
		case format.AddCase("text"), format.AddCase(""):
			l.Formatter = &logrus.TextFormatter{
				DisableQuote:     true,
				DisableTimestamp: false,
				FullTimestamp:    true,
			}
		}

		if unknownFormat {
			l.WithError(format.ToUnknownCaseErr()).Warn("got unknown \"log.format\", falling back to \"text\"")
		}
	}
}

func ForceLevel(level logrus.Level) Option {
	return func(o *options) {
		o.level = &level
	}
}

func ForceFormatter(formatter logrus.Formatter) Option {
	return func(o *options) {
		o.formatter = formatter
	}
}

func WithConfigurator(c configurator) Option {
	return func(o *options) {
		o.c = c
	}
}

func ForceFormat(format string) Option {
	return func(o *options) {
		o.format = format
	}
}

func WithHook(hook logrus.Hook) Option {
	return func(o *options) {
		o.hooks = append(o.hooks, hook)
	}
}

func WithExitFunc(exitFunc func(int)) Option {
	return func(o *options) {
		o.exitFunc = exitFunc
	}
}

func ReportCaller(reportCaller bool) Option {
	return func(o *options) {
		o.reportCaller = reportCaller
	}
}

func UseLogger(l *logrus.Logger) Option {
	return func(o *options) {
		o.l = l
	}
}

func LeakSensitive() Option {
	return func(o *options) {
		o.leakSensitive = true
	}
}

func RedactionText(text string) Option {
	return func(o *options) {
		o.redactionText = text
	}
}

func (c *nullConfigurator) Bool(_ string) bool {
	return false
}

func (c *nullConfigurator) String(_ string) string {
	return ""
}

func newOptions(opts []Option) *options {
	o := new(options)
	o.c = new(nullConfigurator)
	for _, f := range opts {
		f(o)
	}
	return o
}

// New creates a new logger with all the important fields set.
func New(name string, version string, opts ...Option) *Logger {
	o := newOptions(opts)
	return &Logger{
		opts:          opts,
		name:          name,
		version:       version,
		leakSensitive: o.leakSensitive || o.c.Bool("log.leak_sensitive_values"),
		redactionText: stringsx.DefaultIfEmpty(o.redactionText, `Value is sensitive and has been redacted. To see the value set config key "log.leak_sensitive_values = true" or environment variable "LOG_LEAK_SENSITIVE_VALUES=true".`),
		Entry: newLogger(o.l, o).WithFields(logrus.Fields{
			"audience": "application", "service_name": name, "service_version": version}),
	}
}

func NewAudit(name string, version string, opts ...Option) *Logger {
	return New(name, version, opts...).WithField("audience", "audit")
}

func (l *Logger) UseConfig(c configurator) {
	l.leakSensitive = l.leakSensitive || c.Bool("log.leak_sensitive_values")
	l.redactionText = stringsx.DefaultIfEmpty(c.String("log.redaction_text"), l.redactionText)
	o := newOptions(append(l.opts, WithConfigurator(c)))
	setLevel(l.Entry.Logger, o)
	setFormatter(l.Entry.Logger, o)
}

func (l *Logger) ReportError(r *http.Request, code int, err error, args ...interface{}) {
	l.WithError(err).WithRequest(r).WithField("http_response", map[string]interface{}{
		"status_code": code,
	}).Error(args...)
}
