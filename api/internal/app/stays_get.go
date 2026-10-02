package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// GetStay returns a stay with its live quote: at the injected clock while ACTIVE, at check-out after.
func (s *Stays) GetStay(ctx context.Context, c Caller, stayID string) (StayDetail, error) {
	const op = "getStay"
	if err := s.checkRole(op, c); err != nil {
		return StayDetail{}, err
	}
	var out StayDetail
	err := s.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		rec, ok, err := s.repo.StayByID(ctx, tx, stayID)
		if err != nil {
			return fmt.Errorf("stay: %w", err)
		}
		if !ok {
			return ErrNotFound
		}
		if err := s.checkBuilding(ctx, op, c, rec.BuildingID); err != nil {
			return err
		}
		out, err = s.detail(ctx, tx, c, rec)
		return err
	})
	return out, err
}

func (s *Stays) detail(ctx context.Context, tx Tx, c Caller, rec StayRecord) (StayDetail, error) {
	loc, err := s.zone(ctx, tx)
	if err != nil {
		return StayDetail{}, err
	}
	asOf, err := quoteInstant(rec, s.clock.Now())
	if err != nil {
		return StayDetail{}, err
	}
	return detailFor(rec, asOf, loc)
}

// quoteInstant fails closed on a stored status or a CHECKED_OUT stay without a check-out time.
func quoteInstant(rec StayRecord, now time.Time) (time.Time, error) {
	st, err := stay.ParseStatus(rec.Status)
	if err != nil {
		return time.Time{}, fmt.Errorf("stay status: %w", err)
	}
	if st == stay.StatusActive {
		return now, nil
	}
	if rec.CheckOutAt == nil {
		return time.Time{}, fmt.Errorf("stay %s has no check-out time", rec.ID)
	}
	return *rec.CheckOutAt, nil
}
