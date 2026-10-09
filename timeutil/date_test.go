package timeutil

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	t.Parallel()

	got, err := ParseDate("2026-03-09")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	if want := (Date{Year: 2026, Month: time.March, Day: 9}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, bad := range []string{"", "2026-3-9", "2026-02-30", "2026-03-09T00:00:00Z", "09/03/2026"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q) succeeded, want error", bad)
		}
	}
}

func TestDate_String(t *testing.T) {
	t.Parallel()

	if got := NewDate(7, time.January, 2).String(); got != "0007-01-02" {
		t.Errorf("got %q", got)
	}
}

func TestDate_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	type doc struct {
		ShipOn  Date  `json:"ship_on"`
		CloseOn *Date `json:"close_on"`
	}

	in := doc{ShipOn: NewDate(2026, time.December, 31)}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"ship_on":"2026-12-31","close_on":null}`; string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}

	var out doc
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ShipOn != in.ShipOn || out.CloseOn != nil {
		t.Errorf("round trip got %+v", out)
	}

	if err := json.Unmarshal([]byte(`{"ship_on":20261231}`), &out); err == nil {
		t.Error("numeric date accepted, want error")
	}
}

func TestDate_StartUsesLocation(t *testing.T) {
	t.Parallel()

	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	start := NewDate(2026, time.July, 4).Start(chicago)
	if got := start.UTC().Format(time.RFC3339); got != "2026-07-04T05:00:00Z" {
		t.Errorf("got %s", got)
	}
	if got := DateOf(start.UTC()); got != NewDate(2026, time.July, 4) {
		t.Errorf("DateOf in UTC got %v", got)
	}
}

func TestDate_AddDaysAndCompare(t *testing.T) {
	t.Parallel()

	d := NewDate(2028, time.February, 28)
	if got := d.AddDays(1); got != NewDate(2028, time.February, 29) {
		t.Errorf("leap day: got %v", got)
	}
	if got := d.AddDays(2); got != NewDate(2028, time.March, 1) {
		t.Errorf("month rollover: got %v", got)
	}
	if got := NewDate(2026, time.January, 1).AddDays(-1); got != NewDate(2025, time.December, 31) {
		t.Errorf("year rollback: got %v", got)
	}
	if !d.Before(d.AddDays(1)) || !d.AddDays(1).After(d) || d.Before(d) || d.After(d) {
		t.Error("Before/After ordering wrong")
	}
	if !(Date{}).IsZero() || d.IsZero() {
		t.Error("IsZero wrong")
	}
}

func TestDate_Scan(t *testing.T) {
	t.Parallel()

	want := NewDate(2026, time.May, 1)
	for _, src := range []any{
		time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		[]byte("2026-05-01"),
		"2026-05-01",
	} {
		var d Date
		if err := d.Scan(src); err != nil {
			t.Errorf("Scan(%T): %v", src, err)
			continue
		}
		if d != want {
			t.Errorf("Scan(%T) = %v, want %v", src, d, want)
		}
	}

	var d Date
	if err := d.Scan(int64(20260501)); err == nil {
		t.Error("Scan(int64) succeeded, want error")
	}

	v, err := want.Value()
	if err != nil || v != "2026-05-01" {
		t.Errorf("Value() = %v, %v", v, err)
	}
}
