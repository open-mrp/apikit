package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/version"
)

type RespondOption func(http.Header)

func WithHeader(key, value string) RespondOption {
	return func(h http.Header) { h.Set(key, value) }
}

func WithLocation(path string) RespondOption {
	return func(h http.Header) { h.Set("Location", path) }
}

func applyRespondOptions(w http.ResponseWriter, ctx context.Context, code int, opts []RespondOption) {
	w.Header().Set(ContentTypeHeader, "application/json")
	setVersionHeaders(ctx, w.Header())
	rl, ok := appctx.GetRequestLog(ctx)
	if ok && rl != nil {
		w.Header().Set(RequestIDHeader, rl.ID)
	}
	if code == http.StatusUnauthorized {
		w.Header().Set(WwwAuthenticateHeader, "Bearer")
	}
	for _, opt := range opts {
		opt(w.Header())
	}
}

func RespondWithJSON(ctx context.Context, w http.ResponseWriter, code int, payload any, opts ...RespondOption) {
	applyRespondOptions(w, ctx, code, opts)
	var dat []byte
	var err error
	rl, ok := appctx.GetRequestLog(ctx)
	if ok && rl != nil && rl.UserAgent != nil && strings.HasPrefix(*rl.UserAgent, "curl/") {
		dat, err = json.MarshalIndent(payload, "", "  ")
	} else {
		dat, err = json.Marshal(payload)
	}
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		if _, werr := w.Write([]byte(`{"error":{"code":"internal_error","type":"api_error","message":"Something went wrong.","is_transient":false}}`)); werr != nil {
			log.Printf("Error writing fallback JSON error body: %s", werr)
		}
		return
	}
	w.WriteHeader(code)
	if _, err := w.Write(dat); err != nil {
		log.Printf("Error writing response: %s", err)
	}
}

// FileDownload is a response type for endpoints that return a file (e.g. Excel export).
// When the service returns *FileDownload, the handler writes the body with Content-Type and Content-Disposition.
type FileDownload struct {
	ContentType string
	Filename    string
	Body        []byte
}

// RespondWithFile writes a file download response.
func RespondWithFile(ctx context.Context, w http.ResponseWriter, code int, fd *FileDownload, opts ...RespondOption) {
	if fd == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(ContentTypeHeader, fd.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fd.Filename+"\"")
	setVersionHeaders(ctx, w.Header())
	if rl, ok := appctx.GetRequestLog(ctx); ok && rl != nil {
		w.Header().Set(RequestIDHeader, rl.ID)
	}
	for _, opt := range opts {
		opt(w.Header())
	}
	w.WriteHeader(code)
	if _, err := w.Write(fd.Body); err != nil {
		log.Printf("Error writing file response: %s", err)
	}
}

func (*FileDownload) SchemaExample() any {
	return map[string]any{
		"content_type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"filename":     "export.xlsx",
	}
}

// setVersionHeaders names the API version the response is shaped for, and announces a deprecated version's deprecation and sunset (RFC 9745, RFC 8594). Nothing is set when the app registered no versions.
func setVersionHeaders(ctx context.Context, h http.Header) {
	v, ok := appctx.GetAPIVersionFromContext(ctx)
	if !ok {
		supported := version.Supported()
		if len(supported) == 0 {
			return
		}
		v = supported[0]
	}
	h.Set(version.Header(), v.String())
	if !v.DeprecatedAt.IsZero() {
		h.Set(DeprecationHeader, "@"+strconv.FormatInt(v.DeprecatedAt.Unix(), 10))
	}
	if !v.SunsetAt.IsZero() {
		h.Set(SunsetHeader, v.SunsetAt.UTC().Format(http.TimeFormat))
	}
}
