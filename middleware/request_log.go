package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/id"
	"github.com/open-mrp/apikit/logging"
	"github.com/open-mrp/apikit/redact"
	"github.com/open-mrp/apikit/tracing"
	"github.com/open-mrp/apikit/transport"
	"github.com/open-mrp/apikit/version"
)

// RequestLogSaver persists a finished request's log, for example by publishing it to a queue or writing it to a table.
type RequestLogSaver interface {
	Save(ctx context.Context, rl *appctx.RequestLog) error
}

// RouteMatcher finds the route pattern a request reaches, such as router.Router.
type RouteMatcher interface {
	Match(method, path string) (pattern string, public bool, ok bool)
}

// RequestLogConfig configures RequestLog.
type RequestLogConfig struct {
	// Saver (optional) persists each request's log. Without one, only the canonical log line is written.
	Saver RequestLogSaver
	// Routes (optional) names the route pattern of each request, so logs group by endpoint rather than by path.
	Routes RouteMatcher
	// Logger (optional; default: slog.Default()) receives the canonical log line.
	Logger *slog.Logger
	// NewID (optional; default: an "rq_" ID) returns each request's ID, which is also sent back in the Request-ID header.
	NewID func() string
	// TrustedProxyHops (optional; default: 0) is how many proxies in front of the server append to X-Forwarded-For.
	TrustedProxyHops int
	// SkipPaths (optional) are paths not logged, such as a health check. Preflight OPTIONS requests are never logged.
	SkipPaths []string
}

// maxResponseLogSize caps the response body kept on the request log.
const maxResponseLogSize = 256 << 10

var requestLogTracer = tracing.GetTracer("apikit.middleware.request_log")

// RequestLog wraps the whole handler, usually the router: it starts a request log for every request and puts it, and the request ID, on the context; when the request ends it records the status, latency and response body (with sensitive fields redacted), saves the log, and writes one canonical log line.
func RequestLog(cfg RequestLogConfig) func(http.Handler) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	newID := cfg.NewID
	if newID == nil {
		prefix, length := id.ComposePrefix("rq"), id.IDLength19
		newID = func() string {
			v, err := id.GenID(prefix, &length)
			if err != nil {
				panic(err)
			}
			return v
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()
			requestID := newID()

			route, public := r.URL.Path, true
			if cfg.Routes != nil {
				if pattern, isPublic, ok := cfg.Routes.Match(r.Method, r.URL.Path); ok {
					route, public = pattern, isPublic
				}
			}
			userAgent, referrer := r.UserAgent(), r.Referer()
			clientIP := transport.GetClientIP(r, cfg.TrustedProxyHops)
			rl := &appctx.RequestLog{
				ID:              requestID,
				Method:          r.Method,
				Host:            r.Host,
				Path:            r.URL.Path,
				NormalizedRoute: route,
				UserAgent:       &userAgent,
				Referrer:        &referrer,
				ClientIP:        clientIP,
				OccurredAt:      start,
				PublicEndpoint:  public,
			}
			if len(clientIP) > 0 {
				s := clientIP.String()
				rl.ClientIPString = &s
			}
			if v := r.Header.Get(version.Header()); v != "" {
				rl.APIVersion = &v
			}
			if r.URL.RawQuery != "" {
				query := map[string]any{}
				for k, v := range r.URL.Query() {
					if len(v) == 1 {
						query[k] = v[0]
					} else {
						query[k] = v
					}
				}
				if b, err := json.Marshal(query); err == nil {
					s := string(b)
					rl.QueryJSON = &s
				}
			}
			if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
				span.SetAttributes(attribute.String("request.id", requestID))
				traceID := span.SpanContext().TraceID().String()
				rl.TraceID = &traceID
			}

			ctx := appctx.WithRequestID(appctx.WithRequestLog(r.Context(), rl), requestID)
			r = r.WithContext(ctx)
			lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			defer func() {
				if r.Method == http.MethodOptions || slices.Contains(cfg.SkipPaths, r.URL.Path) {
					return
				}
				rl.StatusCode = lrw.statusCode
				rl.LatencyUs = time.Since(start).Microseconds()
				rl.ResponseJSON = responseForLog(lrw, rl.SensitiveResponseFields)
				if rl.Identity == nil {
					rl.Identity, _ = appctx.AnyIdentity(r.Context())
				}

				if cfg.Saver != nil && !rl.SkipSave {
					saveCtx, span := requestLogTracer.Start(ctx, "middleware.request_log.save")
					if err := cfg.Saver.Save(saveCtx, rl); err != nil {
						logger.ErrorContext(ctx, "saving request log failed", "request_id", requestID, "error", err)
					}
					span.End()
				}

				attrs := append([]slog.Attr{
					slog.String("type", logging.TypeCanonicalLogLine),
					slog.String("http_method", r.Method),
					slog.String("http_route", route),
					slog.Int("http_status", lrw.statusCode),
					slog.Float64("duration_ms", float64(rl.LatencyUs)/1000),
				}, logging.RequestAttrs(ctx)...)
				if rl.ErrorCode != nil {
					attrs = append(attrs, slog.String("error_code", *rl.ErrorCode))
				}
				logger.LogAttrs(ctx, slog.LevelInfo, r.Method+" "+route, attrs...)
			}()

			next.ServeHTTP(lrw, r)
		})
	}
}

// responseForLog is the response body to keep: redacted, and replaced by a marker when it was cut off or is not valid JSON, so the log row still saves.
func responseForLog(lrw *loggingResponseWriter, sensitive map[string]bool) *string {
	if len(lrw.body) == 0 {
		return nil
	}
	if !lrw.bodyFull {
		s := fmt.Sprintf(`{"_truncated":true,"_original_size_exceeded":%d}`, maxResponseLogSize)
		return &s
	}
	body := lrw.body
	if len(sensitive) > 0 {
		body = redact.RedactJSON(body, sensitive)
	}
	if len(body) == 0 {
		return nil
	}
	s := string(body)
	if !json.Valid(body) {
		s = fmt.Sprintf(`{"_invalid_json":true,"_original_size":%d}`, len(body))
	}
	return &s
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
	body       []byte
	bodyFull   bool
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	if !lrw.written {
		lrw.statusCode = code
		lrw.ResponseWriter.WriteHeader(code)
		lrw.written = true
	}
}

func (lrw *loggingResponseWriter) Write(data []byte) (int, error) {
	if !lrw.written {
		lrw.WriteHeader(http.StatusOK)
	}
	if remaining := maxResponseLogSize - len(lrw.body); remaining > 0 {
		if len(data) <= remaining {
			lrw.body = append(lrw.body, data...)
			lrw.bodyFull = true
		} else {
			lrw.body = append(lrw.body, data[:remaining]...)
			lrw.bodyFull = false
		}
	} else {
		lrw.bodyFull = false
	}
	return lrw.ResponseWriter.Write(data)
}

// Unwrap lets http.ResponseController reach the underlying writer, e.g. to flush a stream.
func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lrw.ResponseWriter
}
