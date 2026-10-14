package apierror_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/apierror"
)

func roundTrip(t *testing.T, e *apierror.APIError) *apierror.APIError {
	t.Helper()
	b, err := e.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	got, err := apierror.FromJSON(b)
	if err != nil {
		t.Fatalf("FromJSON(%s): %v", b, err)
	}
	return got
}

func TestWireRoundTripKeepsPublicAndInternalFields(t *testing.T) {
	t.Parallel()

	sent := apierror.NewValidationError("2 fields are invalid.",
		apierror.Field("name", apierror.CodeMissingField, "name is required."),
		apierror.Field("lines[1].quantity", apierror.CodeInvalidFormat, "quantity must be positive."),
	)
	sent.InternalMessage = "while creating an order"

	got := roundTrip(t, sent)
	if got.Code != sent.Code || got.Type != sent.Type || got.PublicMessage != sent.PublicMessage || got.Param != "name" {
		t.Errorf("public fields: got %+v", got)
	}
	if !reflect.DeepEqual(got.Errors, sent.Errors) {
		t.Errorf("errors: got %+v, want %+v", got.Errors, sent.Errors)
	}
	if got.InternalMessage != "while creating an order" || got.Status() != sent.Status() {
		t.Errorf("internal message %q status %d", got.InternalMessage, got.Status())
	}
	if !reflect.DeepEqual(got.Object(), sent.Object()) {
		t.Errorf("client object differs after the hop:\n got %+v\nwant %+v", got.Object(), sent.Object())
	}
}

func TestWireRoundTripKeepsCauseAndStack(t *testing.T) {
	t.Parallel()

	root := errors.New("connection reset")
	inner := apierror.NewInternalError(root, "load order")
	sent := apierror.NewInternalError(inner, "create shipment")

	got := roundTrip(t, sent)
	if got.Error() != sent.Error() {
		t.Errorf("Error(): got %q, want %q", got.Error(), sent.Error())
	}
	if got.Stack == "" || got.Stack != sent.Stack {
		t.Error("stack was not carried")
	}
	if !got.IsTransient {
		t.Error("internal_error should stay transient")
	}
}

func TestWireRoundTripDecodesRegisteredDetails(t *testing.T) {
	t.Parallel()

	sent := apierror.New(codeQuotaReached, "The plan allows 3.", apierror.WithDetails(quota{Limit: 3, Used: 3}))
	got := roundTrip(t, sent)
	if q, ok := got.Details.(quota); !ok || q != (quota{Limit: 3, Used: 3}) {
		t.Fatalf("details: got %#v", got.Details)
	}
	if obj := marshal(t, got.Object()); obj["quota"] == nil {
		t.Errorf("details member missing after the hop: %v", obj)
	}
}

func TestWireTypeAndTransienceComeFromTheRegistry(t *testing.T) {
	t.Parallel()

	got, err := apierror.FromJSON([]byte(`{"code":"rate_limited","type":"api_error","message":"slow down","is_transient":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != apierror.TypeInvalidRequest || !got.IsTransient {
		t.Errorf("registered spec should win: type %s transient %v", got.Type, got.IsTransient)
	}
}

func TestWireKeepsUnregisteredCodes(t *testing.T) {
	t.Parallel()

	got, err := apierror.FromJSON([]byte(`{"code":"from_a_newer_service","type":"invalid_request_error","message":"m","is_transient":true,"details":{"a":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "from_a_newer_service" || got.Type != apierror.TypeInvalidRequest || !got.IsTransient {
		t.Errorf("got %+v", got)
	}
	if raw, ok := got.Details.(json.RawMessage); !ok || !strings.Contains(string(raw), `"a"`) {
		t.Errorf("details: got %#v", got.Details)
	}
}

func TestWireNil(t *testing.T) {
	t.Parallel()

	var e *apierror.APIError
	if b, err := e.ToJSON(); b != nil || err != nil {
		t.Errorf("nil error encoded to %s, %v", b, err)
	}
	if _, err := apierror.FromJSON([]byte("not json")); err == nil {
		t.Error("malformed input decoded")
	}
}
