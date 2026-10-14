package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"

	apierror "github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/version"
)

func RespondWithAPIError(ctx context.Context, w http.ResponseWriter, apiErr *apierror.APIError, opts ...RespondOption) {
	if apiErr == nil {
		panic("RespondWithAPIError: apiErr received is nil.")
	}

	rl, hasRL := appctx.GetRequestLog(ctx)
	if hasRL && rl != nil {
		errorCode := string(apiErr.Code)
		rl.ErrorCode = &errorCode
		rl.ErrorMessage = &apiErr.PublicMessage
		// Record the internal chain and stack for every 5xx, not just internal_error: timeouts, service-unavailable, etc. are equally in need of diagnosis in the error alert, and NewAPIError already captures a stack at the origin for all of them.
		if apiErr.Status() >= http.StatusInternalServerError {
			// Record the full internal chain (InternalMessage + wrapped Internal error), not just the top-level message — otherwise the underlying cause (e.g. the real driver error behind "Database request failed for unknown reason.") is lost.
			internalMessage := apiErr.Error()
			rl.InternalErrorMessage = &internalMessage
			// Prefer the stack captured where the error originated (NewInternalError). Fall back to capturing here only if the error carries no origin stack.
			if apiErr.Stack != "" {
				st := apiErr.Stack
				rl.StackTrace = &st
			} else {
				stackTrace := make([]byte, 32768) // 32KB
				length := runtime.Stack(stackTrace, false)
				st := string(stackTrace[:length])
				rl.StackTrace = &st
			}
		}
	}

	status, body := downgradeError(ctx, apiErr)
	RespondWithJSON(ctx, w, status, body, opts...)
}

// downgradeError is the error response in the shape of the caller's API version: as built for the latest version, then through the error downgrades of every registered transformer down to the requested one (version.ErrorDowngrader).
func downgradeError(ctx context.Context, apiErr *apierror.APIError) (int, any) {
	status, resp := apiErr.Status(), apiErr.Response()
	requested, ok := appctx.GetAPIVersionFromContext(ctx)
	if !ok || len(version.Supported()) == 0 || requested.Equal(version.Latest()) {
		return status, resp
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return status, resp
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return status, resp
	}
	return version.TransformError(version.Latest(), requested, status, body)
}
