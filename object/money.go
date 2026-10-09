package object

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Money is an amount in one currency. The amount is a decimal string, so no client loses precision parsing it as a float; the currency is a lowercase ISO 4217 code. It has no id, object or timestamps and is written only through its parent.
type Money struct {
	// The amount, as a decimal string such as "12.50".
	Amount string `json:"amount" validate:"required,decimal"`
	// Three-letter ISO 4217 currency code, lowercase, such as "usd".
	Currency string `json:"currency" validate:"required,len=3,lowercase,alpha"`
}

// NewMoney returns amount in currency, keeping the amount's exact digits.
func NewMoney(amount decimal.Decimal, currency string) Money {
	return Money{Amount: amount.String(), Currency: currency}
}

// NewMoneyFixed returns amount in currency rounded half away from zero to places decimal places, written with exactly that many, as document totals (2 places) are.
func NewMoneyFixed(amount decimal.Decimal, currency string, places int32) Money {
	return Money{Amount: amount.StringFixed(places), Currency: currency}
}

// Decimal parses the amount.
func (m Money) Decimal() (decimal.Decimal, error) {
	d, err := decimal.NewFromString(m.Amount)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("money amount %q is not a decimal", m.Amount)
	}
	return d, nil
}

// SchemaExample returns a representative amount for schema generation.
func (Money) SchemaExample() any {
	return Money{Amount: "125.00", Currency: "usd"}
}
