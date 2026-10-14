package transport

import (
	"testing"

	apierror "github.com/open-mrp/apikit/apierror"
)

type enumErrColor string

func (*enumErrColor) EnumValues() []string { return []string{"red", "blue"} }

type enumErrSection struct {
	Color enumErrColor `json:"color"`
}

type enumErrReq struct {
	Filter  *enumErrColor   `query:"color"`
	Filters []enumErrColor  `query:"colors"`
	Body    *enumErrColor   `json:"body_color"`
	Bodies  []enumErrColor  `json:"body_colors"`
	Section *enumErrSection `json:"section"`
}

// A bad query enum is a 400 parameter; a bad body enum, at any depth, is a 422 field entry.
func TestValidateEnumFields_bodyVersusParameter(t *testing.T) {
	t.Parallel()
	green := enumErrColor("green")
	tests := []struct {
		name      string
		req       enumErrReq
		wantCode  apierror.Code
		wantParam string
	}{
		{"query scalar", enumErrReq{Filter: &green}, apierror.CodeParameterInvalid, "color"},
		{"query list", enumErrReq{Filters: []enumErrColor{"red", green}}, apierror.CodeParameterInvalid, "colors"},
		{"body scalar", enumErrReq{Body: &green}, apierror.CodeValidationFailed, "body_color"},
		{"body list", enumErrReq{Bodies: []enumErrColor{green}}, apierror.CodeValidationFailed, "body_colors"},
		{"body section", enumErrReq{Section: &enumErrSection{Color: green}}, apierror.CodeValidationFailed, "section.color"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateEnumFields(&tt.req)
			if err == nil {
				t.Fatal("want an error")
			}
			if err.Code != tt.wantCode || err.Param != tt.wantParam {
				t.Fatalf("got %s param %q, want %s param %q", err.Code, err.Param, tt.wantCode, tt.wantParam)
			}
			if tt.wantCode == apierror.CodeValidationFailed && (len(err.Errors) != 1 || err.Errors[0].Code != apierror.CodeInvalidFormat) {
				t.Errorf("errors = %+v, want one invalid_format entry", err.Errors)
			}
		})
	}
}
