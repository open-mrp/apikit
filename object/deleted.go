package object

// Deleted is the body of a successful delete: 200 with {id, object, deleted: true}, never an empty body or a 204.
type Deleted struct {
	// The ID of the deleted resource.
	ID string `json:"id" validate:"required"`
	// The deleted resource's object type.
	Object Type `json:"object" validate:"required"`
	// Always true.
	Deleted bool `json:"deleted" validate:"required"`
}

// NewDeleted returns the stub for a deleted resource.
func NewDeleted(id string, objectType Type) *Deleted {
	return &Deleted{ID: id, Object: objectType, Deleted: true}
}
