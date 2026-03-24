package dbconv

import (
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
)

func PtrToOmitNull[T, R any](
	s *T,
	parse func(T) R,
) omitnull.Val[R] {
	if s == nil {
		return omitnull.Val[R]{}
	}

	v := parse(*s)
	return omitnull.From(v)
}

func StrPtrToNullUUID(strPtr *string) omitnull.Val[uuid.UUID] {
	return PtrToOmitNull(strPtr, func(s string) uuid.UUID {
		return uuid.MustParse(s)
	})
}
