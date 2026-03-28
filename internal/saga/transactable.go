package saga

import (
	"context"

	"github.com/stephenafamo/bob"
)

//go:generate minimock -i Transactable -o ./saga_mocks/transactable_mock.go
//go:generate minimock -i BobTransaction -o ./saga_mocks/bob_transaction_mock.go

type Transactable interface {
	WithTx(ctx context.Context) (bob.Transaction, error)
}

// BobTransaction only for test.
type BobTransaction interface {
	bob.Transaction
}
