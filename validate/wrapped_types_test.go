package validate

import (
	"reflect"
	"testing"

	"github.com/open-mrp/apikit/field"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wrappedAddress struct {
	Name    string                 `json:"name" validate:"required"`
	Email   field.Optional[string] `json:"email,omitzero" validate:"omitempty,custom_email"`
	Country string                 `json:"country" validate:"required,max=2"`
}

type wrappedAddressRequest struct {
	BillTo field.Optional[wrappedAddress] `json:"bill_to,omitzero"`
}

func init() {
	RegisterWrappedTypes(field.Optional[wrappedAddress]{})
}

func TestValidate_ChecksTheFieldsInsideASetOptionalStruct(t *testing.T) {
	t.Parallel()

	assert.Nil(t, Validate(&wrappedAddressRequest{}), "an unset wrapper has nothing to check")
	assert.Nil(t, Validate(&wrappedAddressRequest{BillTo: field.Some(wrappedAddress{Name: "Dock", Country: "US"})}))

	apiErr := Validate(&wrappedAddressRequest{BillTo: field.Some(wrappedAddress{Name: "Dock", Country: "USA"})})
	require.NotNil(t, apiErr)
	assert.Equal(t, "bill_to.country", firstParam(apiErr))

	apiErr = Validate(&wrappedAddressRequest{BillTo: field.Some(wrappedAddress{Country: "US"})})
	require.NotNil(t, apiErr)
	assert.Equal(t, "bill_to.name", firstParam(apiErr))

	apiErr = Validate(&wrappedAddressRequest{BillTo: field.Some(wrappedAddress{Name: "Dock", Email: field.Some("not-an-address"), Country: "US"})})
	require.NotNil(t, apiErr, "a wrapper inside the wrapped struct is checked too")
	assert.Equal(t, "bill_to.email", firstParam(apiErr))
}

type foundClient struct {
	Name    string                 `json:"name" validate:"required,max=10"`
	Version field.Optional[string] `json:"version,omitzero" validate:"omitempty,max=5"`
}

type foundSection struct {
	Client field.Optional[foundClient] `json:"client,omitzero"`
}

type foundRequest struct {
	Client   field.Optional[foundClient] `json:"client,omitzero"`
	Sections []foundSection              `json:"sections,omitzero" validate:"dive"`
	Self     *foundRequest               `json:"self,omitzero"`
}

// Optional sections are found and checked without registering their types by hand, at any depth and through slices; a type that refers to itself does not loop.
func TestRegisterWrappedTypesIn_FindsNestedOptionalStructs(t *testing.T) {
	RegisterWrappedTypesIn(reflect.TypeFor[foundRequest]())
	RegisterWrappedTypesIn(reflect.TypeFor[foundRequest]()) // idempotent

	assert.Nil(t, Validate(&foundRequest{}))
	assert.Nil(t, Validate(&foundRequest{Client: field.Some(foundClient{Name: "claude"})}))

	apiErr := Validate(&foundRequest{Client: field.Some(foundClient{})})
	require.NotNil(t, apiErr, "a required field inside a set optional section is enforced")
	assert.Equal(t, "client.name", firstParam(apiErr))

	apiErr = Validate(&foundRequest{Client: field.Some(foundClient{Name: "claude", Version: field.Some("1.2.3.4")})})
	require.NotNil(t, apiErr)
	assert.Equal(t, "client.version", firstParam(apiErr))

	apiErr = Validate(&foundRequest{Sections: []foundSection{{Client: field.Some(foundClient{Name: "far too long a name"})}}})
	require.NotNil(t, apiErr, "a section inside a slice (with dive) is found too")
}
