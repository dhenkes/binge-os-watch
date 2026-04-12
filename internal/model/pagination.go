package model

// PageRequest holds pagination input parameters.
type PageRequest struct {
	PageSize  int
	PageToken string
}

// PageResponse wraps a paginated result set.
type PageResponse[T any] struct {
	Items         []T    `json:"items"`
	NextPageToken string `json:"next_page_token,omitempty"`
	TotalSize     int    `json:"total_size"`
}

// DefaultPageSize is used when the client doesn't specify a page size.
const DefaultPageSize = 20

// MaxPageSize caps the maximum items per page.
const MaxPageSize = 100

// Normalize ensures page size is within bounds.
func (p PageRequest) Normalize() PageRequest {
	if p.PageSize <= 0 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
	return p
}

// EnsureItems guarantees Items is a non-nil slice so JSON encodes as [] not null.
func (p *PageResponse[T]) EnsureItems() {
	if p.Items == nil {
		p.Items = []T{}
	}
}
