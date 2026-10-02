package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

const (
	movementPageSize = 50
	maxStocktake     = 500
	kindCount        = "COUNT"
	resultDeleted    = "DELETED"
	resultStopped    = "STOPPED_SELLING"
	stocktakePrefix  = "st"
)

var movementKinds = map[string]bool{"OPENING": true, "IN": true, "SALE": true, "COUNT": true, "ADJUST": true}

// WithAlerts turns on the owner alerts of stocktakes.
func (s *Setup) WithAlerts(a AlertWriter) *Setup { s.alerts = a; return s }

func encodeMovementCursor(c MovementCursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.At.UTC().Format(time.RFC3339Nano) + "|" + c.ID))
}

func decodeMovementCursor(s string) (MovementCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	at, id, ok := strings.Cut(string(raw), "|")
	if err != nil || !ok || id == "" {
		return MovementCursor{}, &ValidationError{"cursor", "not a cursor from this list"}
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return MovementCursor{}, &ValidationError{"cursor", "not a cursor from this list"}
	}
	return MovementCursor{At: t, ID: id}, nil
}

// ListMovements pages an item's stock history, newest first. next is empty on the last page.
func (s *Setup) ListMovements(ctx context.Context, c Caller, code string, kind, cursor *string) ([]MovementRow, string, error) {
	if err := s.authz.Check("listStockMovements", c.Role, access.EDIT); err != nil {
		return nil, "", err
	}
	if kind != nil && !movementKinds[*kind] {
		return nil, "", &ValidationError{"kind", "unknown movement kind"}
	}
	var after *MovementCursor
	if cursor != nil && *cursor != "" {
		cur, err := decodeMovementCursor(*cursor)
		if err != nil {
			return nil, "", err
		}
		after = &cur
	}
	var rows []MovementRow
	next := ""
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		item, ok, err := s.repo.Service(ctx, tx, code)
		if err != nil || !ok {
			return errOr(err, ErrNotFound)
		}
		if rows, err = s.repo.Movements(ctx, tx, item.ID, kind, after, movementPageSize+1); err != nil {
			return err
		}
		if len(rows) > movementPageSize {
			rows = rows[:movementPageSize]
			next = encodeMovementCursor(MovementCursor{At: rows[len(rows)-1].At, ID: rows[len(rows)-1].ID})
		}
		return nil
	})
	return rows, next, err
}

// RemoveService deletes an item that was never sold and stops selling one that was: old bills keep its name and price.
func (s *Setup) RemoveService(ctx context.Context, c Caller, key, code string) (string, error) {
	if err := s.authz.Check("removeService", c.Role, access.EDIT); err != nil {
		return "", err
	}
	if err := s.validKey(key); err != nil {
		return "", err
	}
	var out string
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		route := "POST /v1/owner/services/" + code + "/remove"
		out, err = idempotent(ctx, s.idem, tx, route, key, map[string]string{"code": code}, func() (string, error) {
			item, ok, err := s.repo.Service(ctx, tx, code)
			if err != nil || !ok {
				return "", errOr(err, ErrNotFound)
			}
			sold, err := s.repo.ServiceHasSales(ctx, tx, item.ID)
			if err != nil {
				return "", err
			}
			result := resultDeleted
			if sold {
				off := false
				result, err = resultStopped, s.repo.UpdateService(ctx, tx, item.ID, ServicePatch{OnSale: &off})
			} else {
				err = s.repo.DeleteService(ctx, tx, item.ID)
			}
			if err != nil {
				return "", err
			}
			return result, s.record(ctx, tx, c, "SERVICE_REMOVED", "service", item.ID, map[string]any{"result": result})
		})
		return err
	})
	return out, err
}

// StocktakeLine is one counted item.
type StocktakeLine struct {
	ServiceCode string
	Counted     int64
}

// StocktakeDifference is a line whose count differs from the system.
type StocktakeDifference struct {
	ServiceCode     string `json:"serviceCode"`
	System, Counted int64
}

// StocktakeResult is what a stocktake changed; ValueDifference is counted minus system at the latest unit cost.
type StocktakeResult struct {
	ID              string
	Differences     []StocktakeDifference
	ValueDifference int64
}

// maxLevel is the highest building level of the caller: EDIT_ANY asks for EDIT on at least one building.
func maxLevel(c Caller) access.Level {
	best := access.NONE
	if c.Role == access.RoleOwner {
		return access.EDIT
	}
	for _, l := range c.Levels {
		if l > best {
			best = l
		}
	}
	return best
}

// CreateStocktake sets stock to the counted quantities with COUNT movements for the differences and alerts the owner.
func (s *Setup) CreateStocktake(ctx context.Context, c Caller, key string, lines []StocktakeLine, note *string) (StocktakeResult, error) {
	if err := s.authz.Check("createStocktake", c.Role, maxLevel(c)); err != nil {
		return StocktakeResult{}, err
	}
	if err := s.validKey(key); err != nil {
		return StocktakeResult{}, err
	}
	if len(lines) == 0 || len(lines) > maxStocktake {
		return StocktakeResult{}, &ValidationError{"lines", "between 1 and 500 lines"}
	}
	seen := map[string]bool{}
	for i, l := range lines {
		if l.Counted < 0 || l.Counted > maxStock || strings.TrimSpace(l.ServiceCode) == "" || seen[l.ServiceCode] {
			return StocktakeResult{}, &ValidationError{fmt.Sprintf("lines[%d]", i), "a code once, and a count that is not negative"}
		}
		seen[l.ServiceCode] = true
	}
	var out StocktakeResult
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		var err error
		route := "POST /v1/stocktakes"
		out, err = idempotent(ctx, s.idem, tx, route, key, map[string]any{"lines": lines, "note": note}, func() (StocktakeResult, error) {
			return s.stocktake(ctx, tx, c, lines, note)
		})
		return err
	})
	return out, err
}

func (s *Setup) stocktake(ctx context.Context, tx Tx, c Caller, lines []StocktakeLine, note *string) (StocktakeResult, error) {
	res := StocktakeResult{ID: s.ids.New(stocktakePrefix), Differences: []StocktakeDifference{}}
	now := s.clock.Now()
	sorted := append([]StocktakeLine(nil), lines...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ServiceCode < sorted[j].ServiceCode }) // fixed lock order
	var items []struct {
		item ServiceItem
		diff int64
	}
	for i, l := range sorted {
		item, ok, err := s.repo.Service(ctx, tx, l.ServiceCode)
		if err != nil {
			return res, err
		}
		if !ok {
			return res, &ValidationError{fmt.Sprintf("lines[%d].serviceCode", i), "unknown item"}
		}
		if d := l.Counted - item.Stock; d != 0 {
			items = append(items, struct {
				item ServiceItem
				diff int64
			}{item, d})
		}
	}
	var total int64
	for _, x := range items {
		if x.item.LatestUnitCost != nil {
			v, err := money.Mul(money.Vnd(*x.item.LatestUnitCost), abs64(x.diff))
			if err != nil {
				return res, fmt.Errorf("stocktake value: %w", err)
			}
			if x.diff < 0 {
				total -= v.Int64()
			} else {
				total += v.Int64()
			}
		}
	}
	res.ValueDifference = total
	if err := s.repo.InsertStocktake(ctx, tx, res.ID, c.UserID, note, total, now); err != nil {
		return res, err
	}
	for _, x := range items {
		ref := res.ID
		m := StockMovement{ID: s.ids.New(movementPrefix), ServiceID: x.item.ID, Kind: kindCount, Quantity: x.diff, Ref: &ref, ActorID: &c.UserID, At: now}
		if err := s.repo.InsertMovement(ctx, tx, m); err != nil {
			return res, err
		}
		if _, err := s.repo.AddStock(ctx, tx, x.item.ID, x.diff, nil); err != nil {
			return res, err
		}
		res.Differences = append(res.Differences, StocktakeDifference{ServiceCode: x.item.Code, System: x.item.Stock, Counted: x.item.Stock + x.diff})
	}
	if len(items) > 0 && s.alerts != nil {
		amount := abs64(total)
		err := s.alerts.Raise(ctx, tx, AlertDraft{ID: s.ids.New(alertIDPrefix), Kind: AlertStocktakeDifference, By: c.UserID, Amount: &amount,
			Details: map[string]string{"stocktake": res.ID, "items": fmt.Sprint(len(items))}})
		if err != nil {
			return res, err
		}
	}
	return res, s.record(ctx, tx, c, "STOCKTAKE_CREATED", "stocktake", res.ID, map[string]any{"differences": len(items)})
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
