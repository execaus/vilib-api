package saga

import (
	"context"

	"github.com/stephenafamo/bob"
)

//go:generate mockgen -source=./transactable.go -destination=./mocks/transactable.go -package=mock_saga
type Transactable interface {
	WithTx(ctx context.Context) (bob.Transaction, error)
}

// BobTransaction only for test.
type BobTransaction interface {
	bob.Transaction
}
