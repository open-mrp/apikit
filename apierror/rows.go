package apierror

import (
	"fmt"
	"strconv"
)

// RowError pairs one row of a bulk request with the failure it produced, as a job result reports it.
type RowError struct {
	// Index is the zero-based row of the request.
	Index int
	// Error is the same error object a synchronous response carries.
	Error ErrorObject
}

// RowErrors collects the failures across a bulk request's rows.
type RowErrors struct {
	entries []RowError
	errs    []*APIError
}

// Add records the failure one row produced.
func (r *RowErrors) Add(index int, err *APIError) {
	r.entries = append(r.entries, RowError{Index: index, Error: err.Object()})
	r.errs = append(r.errs, err)
}

// AddField records a row's field failure. param is relative to the row, e.g. "sku".
func (r *RowErrors) AddField(index int, param string, code Code, message string) {
	r.Add(index, NewFieldError(param, code, message))
}

// Any reports whether any row failed.
func (r *RowErrors) Any() bool {
	return len(r.entries) > 0
}

// Entries returns the collected failures, for a job's per-row results.
func (r *RowErrors) Entries() []RowError {
	return r.entries
}

// Summary renders the failures as the single 422 a synchronous bulk request returns, or nil when no row failed. Each field failure is listed with its param prefixed by the row, e.g. "rows[3].sku"; listParam names the request's array of rows.
func (r *RowErrors) Summary(listParam string) *APIError {
	if len(r.errs) == 0 {
		return nil
	}
	var fields []FieldError
	for i, err := range r.errs {
		prefix := listParam + "[" + strconv.Itoa(r.entries[i].Index) + "]"
		if len(err.Errors) == 0 {
			fields = append(fields, Field(prefix, err.Code, err.PublicMessage))
			continue
		}
		for _, fe := range err.Errors {
			param := prefix
			if fe.Param != "" {
				param += "." + fe.Param
			}
			fields = append(fields, Field(param, fe.Code, fe.Message))
		}
	}
	return NewValidationError(fmt.Sprintf("%d of the %s are invalid.", len(r.errs), listParam), fields...)
}
