package postgres

import (
	"fmt"

	"github.com/pcaokhai/stayguard/api/internal/app"
)

// pgTx recovers the adapter transaction from the opaque port handle.
func pgTx(tx app.Tx) (Tx, error) {
	t, ok := tx.(Tx)
	if !ok {
		return Tx{}, fmt.Errorf("transaction was not created by the postgres unit of work (%T)", tx)
	}
	return t, nil
}
