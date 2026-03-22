package saga

import (
	"context"

	"github.com/stephenafamo/bob"
)

type Transactable interface {
	WithTx(ctx context.Context) (bob.Transaction, error)
}
