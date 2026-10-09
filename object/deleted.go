package object

// Confirms that a resource was deleted.
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
