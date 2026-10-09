package validate

import "github.com/open-mrp/apikit/apierror"

// firstParam is the param of the first failing field, where the old error carried its single top-level param.
func firstParam(err *apierror.APIError) string {
	if err == nil || len(err.Errors) == 0 {
		return ""
	}
	return err.Errors[0].Param
}
