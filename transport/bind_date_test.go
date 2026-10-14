package transport

import (
	"testing"
	"time"

	apierror "github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/timeutil"
)

func TestBindFromQuery_date(t *testing.T) {
	t.Parallel()

	type req struct {
		On      timeutil.Date   `query:"on"`
		AfterOn *timeutil.Date  `query:"after_on"`
		Days    []timeutil.Date `query:"days"`
	}
	dst := &req{}
	if err := BindFromQuery(mustParseURL(t, "/x?on=2026-03-09&after_on=2026-02-28&days[]=2026-01-01&days[]=2026-01-02"), dst); err != nil {
		t.Fatal(err)
	}
	if dst.On != timeutil.NewDate(2026, time.March, 9) {
		t.Errorf("on = %v", dst.On)
	}
	if dst.AfterOn == nil || *dst.AfterOn != timeutil.NewDate(2026, time.February, 28) {
		t.Errorf("after_on = %v", dst.AfterOn)
	}
	if len(dst.Days) != 2 || dst.Days[1] != timeutil.NewDate(2026, time.January, 2) {
		t.Errorf("days = %v", dst.Days)
	}

	for _, bad := range []string{"/x?on=2026-02-30", "/x?on=2026-03-09T00:00:00Z", "/x?on=tomorrow"} {
		err := BindFromQuery(mustParseURL(t, bad), &req{})
		requireAPIErr(t, err, apierror.CodeParameterInvalid, "on")
	}
}
