package object

import (
	"time"

	"github.com/open-mrp/apikit/apierror"
)

// Object types of async jobs and their per-row results.
const (
	TypeAsyncJob       Type = "async_job"
	TypeAsyncJobResult Type = "async_job_result"
)

// AsyncJobStatus is how far an async job has got.
type AsyncJobStatus string

const (
	// AsyncJobStatusQueued means the job is accepted and waiting to run.
	AsyncJobStatusQueued AsyncJobStatus = "queued"
	// AsyncJobStatusRunning means the job is running.
	AsyncJobStatusRunning AsyncJobStatus = "running"
	// AsyncJobStatusCompleted means every row was processed, which is not the same as every row succeeding: read each result's status.
	AsyncJobStatusCompleted AsyncJobStatus = "completed"
	// AsyncJobStatusFailed means the job as a whole failed; error says why.
	AsyncJobStatusFailed AsyncJobStatus = "failed"
	// AsyncJobStatusCanceled means the job was canceled before it finished.
	AsyncJobStatusCanceled AsyncJobStatus = "canceled"
)

// IsValid reports whether s is a known status.
func (s AsyncJobStatus) IsValid() bool {
	switch s {
	case AsyncJobStatusQueued, AsyncJobStatusRunning, AsyncJobStatusCompleted, AsyncJobStatusFailed, AsyncJobStatusCanceled:
		return true
	default:
		return false
	}
}

// EnumValues lists every status, for schema generation.
func (AsyncJobStatus) EnumValues() []string {
	return []string{
		string(AsyncJobStatusQueued), string(AsyncJobStatusRunning), string(AsyncJobStatusCompleted),
		string(AsyncJobStatusFailed), string(AsyncJobStatusCanceled),
	}
}

// IsFinal reports whether the job will not change again.
func (s AsyncJobStatus) IsFinal() bool {
	return s == AsyncJobStatusCompleted || s == AsyncJobStatusFailed || s == AsyncJobStatusCanceled
}

// Work a long-running action accepted to run in the background.
//
// Poll it at the URL in the 202 response's `Location` header until `status` is `completed`, `failed` or `canceled`.
type AsyncJob[R any] struct {
	// Async job ID.
	ID string `json:"id" validate:"required"`
	// Always "async_job".
	Object Type `json:"object" validate:"required,enum=async_job"`
	// The kind of work, such as `bulk_create`.
	Type string `json:"type" validate:"required"`
	// The kind of resource the work is on, as an object type, so a job that produced nothing still says what it was for.
	ResourceType *Type `json:"resource_type"`
	// How far the job has got.
	//
	// `completed` means the work was processed, not that every row succeeded: read each entry's own `status` in `results`.
	Status AsyncJobStatus `json:"status" validate:"required"`
	// One entry per submitted row, saying what became of it. Provisional until `status` is final.
	Results *List[AsyncJobResult[R]] `json:"results"`
	// Why the job as a whole failed, in the same shape an error response carries. A row rejected on its own reports its error on its result instead, so this stays null when only some rows failed.
	Error *apierror.ErrorObject `json:"error"`
	// When the job began running.
	StartedAt *time.Time `json:"started_at"`
	// When the job finished processing, whether or not every row succeeded.
	CompletedAt *time.Time `json:"completed_at"`
	// When the job failed.
	FailedAt *time.Time `json:"failed_at"`
	// When the job was canceled.
	CanceledAt *time.Time `json:"canceled_at"`
	// When the job was created.
	CreatedAt time.Time `json:"created_at" validate:"required"`
	// When the job last changed.
	UpdatedAt time.Time `json:"updated_at" validate:"required"`
}

// AsyncJobResultStatus is what became of one row.
type AsyncJobResultStatus string

const (
	// AsyncJobResultCreated means the row produced a new resource.
	AsyncJobResultCreated AsyncJobResultStatus = "created"
	// AsyncJobResultUpdated means the row updated an existing resource.
	AsyncJobResultUpdated AsyncJobResultStatus = "updated"
	// AsyncJobResultFailed means the row was rejected and wrote nothing.
	AsyncJobResultFailed AsyncJobResultStatus = "failed"
)

// IsValid reports whether s is a known row status.
func (s AsyncJobResultStatus) IsValid() bool {
	switch s {
	case AsyncJobResultCreated, AsyncJobResultUpdated, AsyncJobResultFailed:
		return true
	default:
		return false
	}
}

// EnumValues lists every row status, for schema generation.
func (AsyncJobResultStatus) EnumValues() []string {
	return []string{string(AsyncJobResultCreated), string(AsyncJobResultUpdated), string(AsyncJobResultFailed)}
}

// What became of one submitted row: the resource it produced, or why it was rejected.
type AsyncJobResult[R any] struct {
	// Always "async_job_result".
	Object Type `json:"object" validate:"required,enum=async_job_result"`
	// Zero-based row of the request.
	Index int `json:"index"`
	// What became of the row.
	Status AsyncJobResultStatus `json:"status" validate:"required"`
	// The resource the row produced or updated. Null when the row failed.
	Resource *R `json:"resource"`
	// Resources produced alongside the row's own, such as the lines of a created order.
	SubResources *List[R] `json:"sub_resources"`
	// Why the row was rejected. Null unless `status` is `failed`.
	Error *apierror.ErrorObject `json:"error"`
}

// NewAsyncJobResult records a row that produced or updated resource.
func NewAsyncJobResult[R any](index int, status AsyncJobResultStatus, resource *R) AsyncJobResult[R] {
	return AsyncJobResult[R]{Object: TypeAsyncJobResult, Index: index, Status: status, Resource: resource}
}

// NewFailedAsyncJobResult records a row rejected with err.
func NewFailedAsyncJobResult[R any](index int, err *apierror.APIError) AsyncJobResult[R] {
	obj := err.Object()
	return AsyncJobResult[R]{Object: TypeAsyncJobResult, Index: index, Status: AsyncJobResultFailed, Error: &obj}
}
