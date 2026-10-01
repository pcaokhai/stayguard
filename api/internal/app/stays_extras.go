package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

const (
	auditExtrasAdded = "stay.extras_added"
	routeExtrasFmt   = "POST /v1/stays/%s/extras"
)

// extrasRoute includes the stay id so one key cannot replay another stay's answer.
func extrasRoute(stayID string) string { return fmt.Sprintf(routeExtrasFmt, stayID) }

type extraLineJSON struct {
	ServiceCode string `json:"serviceCode"`
	Quantity    int    `json:"quantity"`
}

// extrasHash is over the merged lines, so splitting or reordering the same request replays it.
func extrasHash(lines []stay.ExtraLine) string {
	items := make([]extraLineJSON, len(lines))
	for i, l := range lines {
		items[i] = extraLineJSON{l.ServiceCode, l.Quantity}
	}
	body, _ := json.Marshal(items) // plain fields only: cannot fail
	return RequestHash(body)
}

// AddExtras adds extras to an ACTIVE stay. The order is fixed: role, key, input, then in one unit of work
// the stay lookup, EDIT on its building, the idempotency key and the locked write. replayed is true when
// the stored answer of an earlier identical call is returned.
func (b *Billing) AddExtras(ctx context.Context, c Caller, stayID, retryID string, items []stay.ExtraInput) (StayDetail, bool, error) {
	const op = "addStayExtras"
	if err := b.checkRole(op, c); err != nil {
		return StayDetail{}, false, err
	}
	if err := checkKey(retryID); err != nil {
		return StayDetail{}, false, err
	}
	lines, err := stay.ValidateExtras(items)
	if err != nil {
		return StayDetail{}, false, fmt.Errorf("extras input: %w", err)
	}
	var out StayDetail
	var replayed bool
	err = b.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		route := extrasRoute(stayID)
		oc, err := b.begin(ctx, tx, op, c, stayID, route, retryID, extrasHash(lines))
		if err != nil {
			return err
		}
		if oc.Replay {
			replayed = true
			return json.Unmarshal(oc.Body, &out)
		}
		if out, err = b.addLocked(ctx, tx, c, stayID, items, lines); err != nil {
			return err
		}
		return b.complete(ctx, tx, route, retryID, statusOK, out)
	})
	return out, replayed, err
}

// addLocked runs under the stay row lock: requires ACTIVE, takes stock in code order and inserts the rows.
func (b *Billing) addLocked(ctx context.Context, tx Tx, c Caller, stayID string, items []stay.ExtraInput, lines []stay.ExtraLine) (StayDetail, error) {
	rec, ok, err := b.stays.LockStay(ctx, tx, stayID)
	if err != nil {
		return StayDetail{}, fmt.Errorf("lock stay: %w", err)
	}
	if !ok {
		return StayDetail{}, ErrNotFound
	}
	status, err := stay.ParseStatus(rec.Status)
	if err != nil {
		return StayDetail{}, fmt.Errorf("stay status: %w", err)
	}
	if err := stay.RequireActive(status); err != nil {
		return StayDetail{}, err
	}
	rows, err := b.resolve(ctx, tx, items, lines)
	if err != nil {
		return StayDetail{}, err
	}
	added, err := b.take(ctx, tx, stayID, lines, rows)
	if err != nil {
		return StayDetail{}, err
	}
	if err := b.auditAppend(ctx, tx, c, auditExtrasAdded, entityStay, stayID, map[string]any{"stayId": stayID, "items": added}); err != nil {
		return StayDetail{}, err
	}
	fresh, ok, err := b.stays.StayByID(ctx, tx, stayID)
	if err != nil {
		return StayDetail{}, fmt.Errorf("reload stay: %w", err)
	}
	if !ok {
		return StayDetail{}, ErrNotFound
	}
	return b.detail(ctx, tx, c, fresh)
}

// resolve maps each merged line to its service; unknown codes are a field error on the first item that named them.
func (b *Billing) resolve(ctx context.Context, tx Tx, items []stay.ExtraInput, lines []stay.ExtraLine) (map[string]ServiceRow, error) {
	codes := make([]string, len(lines))
	for i, l := range lines {
		codes[i] = l.ServiceCode
	}
	found, err := b.services.ByCodes(ctx, tx, codes)
	if err != nil {
		return nil, fmt.Errorf("services by code: %w", err)
	}
	rows := make(map[string]ServiceRow, len(found))
	for _, r := range found {
		rows[r.Code] = r
	}
	var errs []stay.FieldError
	seen := map[string]bool{}
	for i, it := range items {
		code := strings.TrimSpace(it.ServiceCode)
		if _, ok := rows[code]; !ok && !seen[code] {
			errs = append(errs, stay.FieldError{Path: fmt.Sprintf("items[%d].serviceCode", i), Code: stay.CodeUnknown})
		}
		seen[code] = true
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("extras input: %w", stay.NewValidationError(errs))
	}
	return rows, nil
}

// addedItem is one audit line: ids and amounts only.
type addedItem struct {
	ServiceID string `json:"serviceId"`
	Quantity  int64  `json:"quantity"`
	Unit      int64  `json:"unit"`
	Amount    int64  `json:"amount"`
}

// take decrements stock then inserts the extras row, line by line in code order. The unit amount is the
// price the guarded decrement returned. Any error rolls the whole unit of work back, so partial stock is never kept.
func (b *Billing) take(ctx context.Context, tx Tx, stayID string, lines []stay.ExtraLine, rows map[string]ServiceRow) ([]addedItem, error) {
	now := storedTime(b.clock.Now())
	added := make([]addedItem, 0, len(lines))
	for _, l := range lines {
		svc, qty := rows[l.ServiceCode], int64(l.Quantity)
		unit, err := b.services.DecrementStock(ctx, tx, svc.ID, qty)
		if err != nil {
			return nil, fmt.Errorf("take stock: %w", err) // keeps stay.ErrInsufficientStock visible to errors.Is
		}
		amount, err := money.Mul(money.Vnd(unit), qty)
		if err != nil {
			return nil, fmt.Errorf("extra amount: %w", err)
		}
		x := NewExtra{ID: b.ids.New(extraIDPrefix), StayID: stayID, ServiceID: svc.ID, Quantity: qty, UnitAmount: unit,
			Amount: amount.Int64(), CreatedAt: now}
		if err := b.stays.InsertExtra(ctx, tx, x); err != nil {
			return nil, fmt.Errorf("insert extra: %w", err)
		}
		added = append(added, addedItem{svc.ID, qty, unit, amount.Int64()})
	}
	return added, nil
}
