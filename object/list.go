package object

// TypeList is the object type of every paginated list.
const TypeList Type = "list"

// ListRequest holds the paging parameters every list endpoint accepts. Embed it in a list request. Search (q) and filters are declared per endpoint, since each documents its own.
type ListRequest struct {
	// Opaque cursor marking where the page starts.
	//
	// Take it from a previous response's `next_page_url` or `previous_page_url`. Omit to start from the first page.
	Cursor *string `query:"cursor"`
	// Maximum number of results to return, from 1 to 100.
	Limit int32 `query:"limit" default:"25" validate:"min=1,max=100"`
}

// PageInfo says where a page sits in its result set and how to reach its neighbors.
//
// Follow the URLs rather than assembling cursors: for a list endpoint the URL repeats the original query string with only the cursor swapped, so the same filters, search and page size carry over.
type PageInfo struct {
	// Relative URL of the next page, or null on the last page.
	NextPageURL *string `json:"next_page_url"`
	// Relative URL of the previous page, or null on the first page.
	PreviousPageURL *string `json:"previous_page_url"`
	// Whether results exist after this page.
	HasNextPage bool `json:"has_next_page"`
	// Whether results exist before this page.
	HasPreviousPage bool `json:"has_previous_page"`
}

// List is one page of resources.
type List[T any] struct {
	// Always "list".
	Object Type `json:"object" validate:"required,enum=list"`
	// Where this page sits in the result set.
	PageInfo PageInfo `json:"page_info"`
	// The resources on this page.
	Data []T `json:"data" validate:"required"`
}

// NewList returns a page of data. A nil slice becomes an empty list, never null.
func NewList[T any](data []T, pageInfo PageInfo) *List[T] {
	if data == nil {
		data = []T{}
	}
	return &List[T]{Object: TypeList, PageInfo: pageInfo, Data: data}
}

// EmbeddedListLimit is how many items an expanded list inside a resource returns; page_info.next_page_url points at the sub-resource list for the rest.
const EmbeddedListLimit = 10
