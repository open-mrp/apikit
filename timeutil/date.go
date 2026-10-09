package timeutil

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// DateLayout is the wire format of a Date: a calendar day with no time or zone.
const DateLayout = "2006-01-02"

// Date is a business day, the value of an `_on` field. It names a day on the account's calendar rather than an instant, so it carries no time zone; turn it into an instant with Start in the account's location.
//
// The zero Date is not a valid day. Use a pointer or field.Optional for a date that may be absent.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate returns the date, normalizing out-of-range months and days the way time.Date does (January 32 is February 1).
func NewDate(year int, month time.Month, day int) Date {
	return DateOf(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

// DateOf returns the calendar day t falls on in t's own location. Convert t with In first to get the day in another zone.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// ParseDate parses a YYYY-MM-DD string. It rejects days that do not exist, such as 2025-02-30.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD", s)
	}
	return DateOf(t), nil
}

// String returns the date as YYYY-MM-DD.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// IsZero reports whether d is the zero Date.
func (d Date) IsZero() bool {
	return d == Date{}
}

// Start returns the instant the day begins in loc.
func (d Date) Start(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// AddDays returns the date n calendar days later, or earlier when n is negative.
func (d Date) AddDays(n int) Date {
	return NewDate(d.Year, d.Month, d.Day+n)
}

// Before reports whether d is an earlier day than other.
func (d Date) Before(other Date) bool {
	return d.compare(other) < 0
}

// After reports whether d is a later day than other.
func (d Date) After(other Date) bool {
	return d.compare(other) > 0
}

func (d Date) compare(other Date) int {
	switch {
	case d.Year != other.Year:
		return d.Year - other.Year
	case d.Month != other.Month:
		return int(d.Month) - int(other.Month)
	default:
		return d.Day - other.Day
	}
}

// MarshalText encodes the date as YYYY-MM-DD, which also covers query parameters and map keys.
func (d Date) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText parses a YYYY-MM-DD date.
func (d *Date) UnmarshalText(b []byte) error {
	parsed, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalJSON encodes the date as a "YYYY-MM-DD" string.
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON parses a "YYYY-MM-DD" string. A JSON null leaves d unchanged, matching encoding/json's handling of other values.
func (d *Date) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("invalid date: expected a YYYY-MM-DD string")
	}
	return d.UnmarshalText([]byte(s))
}

// Value stores the date in a DATE column.
func (d Date) Value() (driver.Value, error) {
	return d.String(), nil
}

// Scan reads a DATE column, which drivers return as a time.Time (with parseTime) or as text.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		*d = DateOf(v)
		return nil
	case []byte:
		return d.UnmarshalText(v)
	case string:
		return d.UnmarshalText([]byte(v))
	default:
		return fmt.Errorf("cannot scan %T into a Date", src)
	}
}
