package dbconv

import (
	"github.com/aarondl/opt/null"
	"github.com/google/uuid"
)

func NullVarToPtr[T, R any](nv null.Val[T], postFn func(*T) *R) *R {
	if nv.IsNull() {
		return nil
	}
	v := nv.MustGet()

	return postFn(&v)
}

func NullUUIDToStrPtr(val null.Val[uuid.UUID]) *string {
	return NullVarToPtr(val, func(t *uuid.UUID) *string {
		if t == nil {
			return nil
		}
		s := t.String()
		return &s
	})
}
