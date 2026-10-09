// Package testdata holds documented endpoint types for the conformance tests; the doc reader reads only non-test source.
package testdata

// Returns a list of widgets, newest first.
type ListWidgets struct{}

// Deletes a widget. A widget other records use cannot be deleted; archive it instead.
type DeleteWidget struct{}

// This endpoint lists gadgets.
type BadSummary struct{}
