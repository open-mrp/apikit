package apierror_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/open-mrp/apikit/apierror"
)

// An app adds its own codes to the built-in ones at startup, then builds errors with New.
func Example() {
	const codeSeatLimitReached apierror.Code = "example_seat_limit_reached"

	type seats struct {
		Limit int `json:"limit"`
		Used  int `json:"used"`
	}

	apierror.Register(apierror.Spec{
		Code:           codeSeatLimitReached,
		Type:           apierror.TypeInvalidRequest,
		Status:         http.StatusForbidden,
		Description:    "Every seat on the plan is taken. Remove a user or upgrade the plan.",
		DetailsField:   "seats",
		DetailsExample: seats{Limit: 5, Used: 5},
	})

	err := apierror.New(codeSeatLimitReached, "All 5 seats are taken.", apierror.WithDetails(seats{Limit: 5, Used: 5}))

	body, _ := json.Marshal(err.Response())
	fmt.Println(err.Status())
	fmt.Println(string(body))
	// Output:
	// 403
	// {"error":{"type":"invalid_request_error","code":"example_seat_limit_reached","message":"All 5 seats are taken.","param":null,"is_transient":false,"errors":[],"doc_url":"https://docs.example.com/errors/example_seat_limit_reached","seats":{"limit":5,"used":5}}}
}
