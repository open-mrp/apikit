package object_test

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/open-mrp/apikit/object"
	"github.com/open-mrp/apikit/validate"
)

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestNewList_emptyDataIsAnEmptyArray(t *testing.T) {
	t.Parallel()
	got := mustJSON(t, object.NewList[string](nil, object.PageInfo{}))
	want := `{"object":"list","page_info":{"next_page_url":null,"previous_page_url":null,"has_next_page":false,"has_previous_page":false},"data":[]}`
	if got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestDeleted_shape(t *testing.T) {
	t.Parallel()
	got := mustJSON(t, object.NewDeleted("cu_123", "customer"))
	if want := `{"id":"cu_123","object":"customer","deleted":true}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestMoney(t *testing.T) {
	t.Parallel()

	m := object.NewMoneyFixed(decimal.RequireFromString("12.345"), "usd", 2)
	if got := mustJSON(t, m); got != `{"amount":"12.35","currency":"usd"}` {
		t.Errorf("fixed: got %s", got)
	}
	if got := object.NewMoney(decimal.RequireFromString("0.125000"), "eur").Amount; got != "0.125" {
		t.Errorf("exact: got %s", got)
	}
	d, err := m.Decimal()
	if err != nil || !d.Equal(decimal.RequireFromString("12.35")) {
		t.Errorf("Decimal() = %v, %v", d, err)
	}

	for _, bad := range []object.Money{
		{Amount: "12.5", Currency: "USD"},
		{Amount: "12.5", Currency: "us"},
		{Amount: "twelve", Currency: "usd"},
		{Amount: "", Currency: "usd"},
	} {
		if err := validate.Validate(bad); err == nil {
			t.Errorf("%+v passed validation", bad)
		}
	}
	if err := validate.Validate(object.Money{Amount: "-3.10", Currency: "cad"}); err != nil {
		t.Errorf("valid money failed: %v", err)
	}
}

func TestListRequest_limitBounds(t *testing.T) {
	t.Parallel()
	for limit, ok := range map[int32]bool{0: false, 1: true, 25: true, 100: true, 101: false} {
		err := validate.Validate(object.ListRequest{Limit: limit})
		if (err == nil) != ok {
			t.Errorf("limit %d: err = %v, want ok=%v", limit, err, ok)
		}
	}
}
