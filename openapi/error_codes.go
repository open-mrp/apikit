package openapi

import "github.com/open-mrp/apikit/apierror"

// ErrorCode documents one error code, for the API reference's error-code index.
type ErrorCode struct {
	Code        apierror.Code `json:"code"`
	Type        apierror.Type `json:"type"`
	Status      int           `json:"status"`
	Transient   bool          `json:"is_transient"`
	Description string        `json:"description"`
	// DocURL is the code's docs page, or "" when the app set no doc URL builder.
	DocURL string `json:"doc_url,omitempty"`
	// DetailsField names the extra member errors with this code carry, if any.
	DetailsField string `json:"details_field,omitempty"`
}

// ErrorCodes lists every registered error code, the kit's and the app's, sorted by code: the source for an error-code index page, where each code's doc_url lands.
func ErrorCodes() []ErrorCode {
	specs := apierror.Specs()
	out := make([]ErrorCode, len(specs))
	for i, s := range specs {
		out[i] = ErrorCode{
			Code:         s.Code,
			Type:         s.Type,
			Status:       s.Status,
			Transient:    s.Transient,
			Description:  s.Description,
			DocURL:       s.Code.DocURL(),
			DetailsField: s.DetailsField,
		}
	}
	return out
}
