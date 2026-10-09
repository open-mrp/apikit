package apierror_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/apierror"
)

const (
	codeQuotaReached apierror.Code = "test_quota_reached"
	codeCardDeclined apierror.Code = "test_card_declined"
	typeCardError    apierror.Type = "test_card_error"
)

func TestAppDefinedTypeAndCode(t *testing.T) {
	t.Parallel()

	err := apierror.New(codeCardDeclined, "Your card was declined.")
	if err.Type != typeCardError || err.Status() != http.StatusPaymentRequired {
		t.Errorf("type %s status %d", err.Type, err.Status())
	}
	if !slices.Contains(typeCardError.EnumValues(), string(typeCardError)) {
		t.Error("registered type missing from EnumValues")
	}
	defer func() {
		if recover() == nil {
			t.Error("registering a type twice did not panic")
		}
	}()
	apierror.RegisterType(typeCardError)
}

type quota struct {
	Limit int `json:"limit"`
	Used  int `json:"used"`
}

// Registrations are process-wide and made before any test runs, the same way an app makes them at startup.
func TestMain(m *testing.M) {
	apierror.SetDocURL(func(c apierror.Code) string { return "https://docs.example.com/errors/" + string(c) })
	apierror.RegisterType(typeCardError)
	apierror.Register(apierror.Spec{
		Code:        codeCardDeclined,
		Type:        typeCardError,
		Status:      http.StatusPaymentRequired,
		Description: "The card was declined.",
	})
	apierror.Register(apierror.Spec{
		Code:           codeQuotaReached,
		Type:           apierror.TypeInvalidRequest,
		Status:         http.StatusForbidden,
		Description:    "The plan's limit is reached.",
		DetailsField:   "quota",
		DetailsExample: quota{Limit: 3, Used: 3},
	})
	os.Exit(m.Run())
}

func marshal(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
	return out
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestResponse_forge1Shape(t *testing.T) {
	t.Parallel()

	err := apierror.NewNotFoundError("No widget with ID wdg_123.")
	body := marshal(t, err.Response())
	obj, ok := body["error"].(map[string]any)
	if !ok || len(body) != 1 {
		t.Fatalf("body = %v, want only an error object", body)
	}

	if got, want := keys(obj), []string{"code", "doc_url", "errors", "is_transient", "message", "param", "type"}; !slices.Equal(got, want) {
		t.Errorf("members = %v, want %v", got, want)
	}
	if v, has := obj["param"]; !has || v != nil {
		t.Errorf("param = %v (present %v), want null for an error not about one parameter", v, has)
	}
	if obj["code"] != "resource_not_found" || obj["type"] != "invalid_request_error" || obj["message"] != "No widget with ID wdg_123." {
		t.Errorf("object = %v", obj)
	}
	if errs, ok := obj["errors"].([]any); !ok || len(errs) != 0 {
		t.Errorf("errors = %#v, want empty list (never null)", obj["errors"])
	}
	if obj["doc_url"] != "https://docs.example.com/errors/resource_not_found" {
		t.Errorf("doc_url = %v", obj["doc_url"])
	}
	if err.Status() != http.StatusNotFound {
		t.Errorf("status = %d", err.Status())
	}
}

func TestValidationError_listsEveryField(t *testing.T) {
	t.Parallel()

	err := apierror.NewValidationError("2 fields are invalid.",
		apierror.Field("sku", apierror.CodeMissingField, "sku is required."),
		apierror.Field("price.amount", apierror.CodeInvalidFormat, "amount must be a decimal string."),
		apierror.Field("", apierror.CodeInvalidFormat, "Send either a or b."),
	)
	if err.Status() != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", err.Status())
	}

	obj := marshal(t, err.Object())
	errs := obj["errors"].([]any)
	if len(errs) != 3 {
		t.Fatalf("errors = %v", errs)
	}
	first := errs[0].(map[string]any)
	if first["param"] != "sku" || first["code"] != "missing_field" || first["message"] != "sku is required." {
		t.Errorf("errors[0] = %v", first)
	}
	if third := errs[2].(map[string]any); third["param"] != nil {
		t.Errorf("errors[2].param = %v, want null for a failure not tied to a field", third["param"])
	}
	if obj["param"] != "sku" {
		t.Errorf("param = %v, want the first failing field", obj["param"])
	}
}

func TestParam_namesTheOffendingParameter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		err  *apierror.APIError
		want string
	}{
		{apierror.NewParameterInvalidError("limit", "limit must be at most 100."), "limit"},
		{apierror.NewParameterUnknownError("colour", "Unknown parameter colour."), "colour"},
		{apierror.NewExistsError("sku", "An item with SKU A-1 already exists."), "sku"},
		{apierror.NewAPIVersionRequiredError("Example-Version", "1.0"), "Example-Version"},
		{apierror.NewInvalidFormatError("email", "email must be a valid email address."), "email"},
	} {
		obj := marshal(t, tc.err.Object())
		if obj["param"] != tc.want {
			t.Errorf("%s: param = %v, want %q", tc.err.Code, obj["param"], tc.want)
		}
		if tc.err.Status() != http.StatusUnprocessableEntity && len(tc.err.Errors) != 0 {
			t.Errorf("%s: errors = %v, want empty on a non-422", tc.err.Code, tc.err.Errors)
		}
	}
}

func TestDetails_writtenOnlyForDeclaringCode(t *testing.T) {
	t.Parallel()

	withDetails := marshal(t, apierror.New(codeQuotaReached, "Plan limit reached.", apierror.WithDetails(quota{Limit: 3, Used: 3})).Object())
	q, ok := withDetails["quota"].(map[string]any)
	if !ok || q["limit"] != float64(3) || q["used"] != float64(3) {
		t.Errorf("quota = %v", withDetails["quota"])
	}

	without := marshal(t, apierror.New(codeQuotaReached, "Plan limit reached.").Object())
	if v, has := without["quota"]; !has || v != nil {
		t.Errorf("quota = %v (present %v), want null member", v, has)
	}

	other := marshal(t, apierror.NewConflictError("Busy.").Object())
	if _, has := other["quota"]; has {
		t.Error("a code without a details field wrote one")
	}
}

func TestNew_copiesSpec(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		err       *apierror.APIError
		status    int
		typ       apierror.Type
		transient bool
	}{
		{apierror.NewRateLimitedError("Slow down."), http.StatusTooManyRequests, apierror.TypeInvalidRequest, true},
		{apierror.NewIdempotencyInProgressError("k1"), http.StatusConflict, apierror.TypeIdempotency, true},
		{apierror.NewIdempotencyKeyReusedError("k1"), http.StatusUnprocessableEntity, apierror.TypeIdempotency, false},
		{apierror.NewInUseError("Archive it instead."), http.StatusConflict, apierror.TypeInvalidRequest, false},
		{apierror.NewParameterUnknownError("colour", "Unknown parameter colour."), http.StatusBadRequest, apierror.TypeInvalidRequest, false},
		{apierror.NewInternalError(errors.New("boom"), "loading widget"), http.StatusInternalServerError, apierror.TypeAPI, true},
		{apierror.NewClientClosedRequestError(), apierror.StatusClientClosedRequest, apierror.TypeAPI, false},
	} {
		if tc.err.Status() != tc.status || tc.err.Type != tc.typ || tc.err.IsTransient != tc.transient {
			t.Errorf("%s: status %d type %s transient %v, want %d %s %v",
				tc.err.Code, tc.err.Status(), tc.err.Type, tc.err.IsTransient, tc.status, tc.typ, tc.transient)
		}
	}
}

func TestNew_unregisteredCodePanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("New with an unregistered code did not panic")
		}
	}()
	apierror.New("never_registered", "x")
}

func TestRegister_rejectsBadSpecs(t *testing.T) {
	t.Parallel()

	for name, spec := range map[string]apierror.Spec{
		"duplicate":      {Code: apierror.CodeResourceGone, Type: apierror.TypeInvalidRequest, Status: 410, Description: "x"},
		"not snake case": {Code: "Bad-Code", Type: apierror.TypeInvalidRequest, Status: 400, Description: "x"},
		"success status": {Code: "test_ok", Type: apierror.TypeInvalidRequest, Status: 200, Description: "x"},
		"unknown type":   {Code: "test_type", Type: "odd_error", Status: 400, Description: "x"},
		"no description": {Code: "test_nodesc", Type: apierror.TypeInvalidRequest, Status: 400},
		"orphan example": {Code: "test_example", Type: apierror.TypeInvalidRequest, Status: 400, Description: "x", DetailsExample: 1},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: Register did not panic", name)
				}
			}()
			apierror.Register(spec)
		}()
	}
}

func TestSpecs_sortedAndComplete(t *testing.T) {
	t.Parallel()

	specs := apierror.Specs()
	if !slices.IsSortedFunc(specs, func(a, b apierror.Spec) int { return strings.Compare(string(a.Code), string(b.Code)) }) {
		t.Error("Specs not sorted by code")
	}
	codes := apierror.CodeValidationFailed.EnumValues()
	for _, want := range []apierror.Code{apierror.CodeRateLimited, apierror.CodeResourceInUse, apierror.CodeIdempotencyKeyReused, codeQuotaReached} {
		if !slices.Contains(codes, string(want)) {
			t.Errorf("EnumValues missing %s", want)
		}
	}
	for _, removed := range []string{"rate_limit_exceeded", "limit_exceeded", "payment_required"} {
		if slices.Contains(codes, removed) {
			t.Errorf("EnumValues has %s, which is not a kit code", removed)
		}
	}
}

func TestStack_capturedFor5xxOnly(t *testing.T) {
	t.Parallel()

	if apierror.NewNotFoundError("x").Stack != "" {
		t.Error("4xx captured a stack")
	}
	inner := apierror.NewInternalError(errors.New("db down"), "query")
	if !strings.Contains(inner.Stack, "TestStack_capturedFor5xxOnly") {
		t.Error("5xx stack does not include the creating function")
	}
	outer := apierror.NewInternalError(inner, "loading")
	if outer.Stack != inner.Stack {
		t.Error("relayed 5xx did not keep the original stack")
	}
}

func TestError_internalMessagesChain(t *testing.T) {
	t.Parallel()

	inner := apierror.NewInternalError(errors.New("connection refused"), "querying widgets")
	outer := apierror.NewInternalError(inner, "listing widgets")
	if got := outer.InternalMessage; got != "listing widgets: querying widgets" {
		t.Errorf("InternalMessage = %q", got)
	}
	if !errors.Is(outer, inner) {
		t.Error("outer does not unwrap to inner")
	}
	if got := outer.Error(); got != "listing widgets: querying widgets: connection refused" {
		t.Errorf("Error() = %q, want each message once and the root cause", got)
	}
	if strings.Contains(outer.Error(), "Something went wrong") {
		t.Error("Error() leaked the public message")
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()

	if got := apierror.Describe(apierror.NewNotFoundError("No widget.")); got != "resource_not_found: No widget." {
		t.Errorf("public-only error: %q", got)
	}
	if got := apierror.Describe(apierror.NewInternalError(errors.New("boom"), "loading")); got != "loading: boom" {
		t.Errorf("internal error: %q", got)
	}
	if got := apierror.Describe(nil); got != "" {
		t.Errorf("nil: %q", got)
	}
}

func TestHasCode_throughWrapping(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("handler: %w", apierror.NewGoneError("Deleted."))
	if !apierror.HasCode(err, apierror.CodeResourceGone) || apierror.HasCode(err, apierror.CodeResourceNotFound) {
		t.Error("HasCode wrong")
	}
}

func TestRowErrors_summary(t *testing.T) {
	t.Parallel()

	var rows apierror.RowErrors
	if rows.Any() || rows.Summary("items") != nil {
		t.Fatal("empty RowErrors reports failures")
	}
	rows.AddField(0, "sku", apierror.CodeMissingField, "sku is required.")
	rows.Add(3, apierror.NewExistsError("sku", "An item with SKU A-1 already exists."))

	if len(rows.Entries()) != 2 || rows.Entries()[1].Index != 3 || rows.Entries()[1].Error.Code != apierror.CodeResourceExists {
		t.Errorf("entries = %+v", rows.Entries())
	}

	sum := rows.Summary("items")
	if sum.Code != apierror.CodeValidationFailed || sum.PublicMessage != "2 of the items are invalid." {
		t.Errorf("summary = %s %q", sum.Code, sum.PublicMessage)
	}
	want := []apierror.FieldError{
		apierror.Field("items[0].sku", apierror.CodeMissingField, "sku is required."),
		apierror.Field("items[3].sku", apierror.CodeResourceExists, "An item with SKU A-1 already exists."),
	}
	if !slices.Equal(sum.Errors, want) {
		t.Errorf("summary errors = %+v, want %+v", sum.Errors, want)
	}
}

func TestDocURL_nullWhenUnset(t *testing.T) {
	// Not parallel: it swaps the process-wide doc URL builder.
	apierror.SetDocURL(nil)
	defer apierror.SetDocURL(func(c apierror.Code) string { return "https://docs.example.com/errors/" + string(c) })

	obj := marshal(t, apierror.NewNotFoundError("x").Object())
	if v, has := obj["doc_url"]; !has || v != nil {
		t.Errorf("doc_url = %v (present %v), want null", v, has)
	}
}
