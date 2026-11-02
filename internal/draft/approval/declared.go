package approval

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

// DeclaredApproval is a consistent local receipt, not an authenticated hosted
// review. Both lock-record operator roles use its declared Operator label.
type DeclaredApproval struct {
	Operator    string
	Head        string
	CaptureLine string
	Record      codehost.ReceiptRecord
}

// DeclaredPort is distinct from the hosted Port so local consistency cannot
// silently satisfy the hosted authenticated-review predicate.
type DeclaredPort interface {
	DeclaredApproval(context.Context, codehost.ReviewBinding) (DeclaredApproval, error)
}
