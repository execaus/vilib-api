package dbconv

import (
	"github.com/aarondl/opt/null"
)

func NullValToPtr[T any](nv null.Val[T]) *T {
	if nv.IsNull() {
		return nil
	}
	v := nv.MustGet()

	return &v
}
