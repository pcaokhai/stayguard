package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
)

const latestPaymentsLimit = 10

// BuildingRevenue is the paid money of one building for the day, split by method.
type BuildingRevenue struct {
	BuildingID, Name string
	Cash, Transfer   int64
}

// PaymentSummary is one paid payment as the owner sees it.
type PaymentSummary struct {
	PaymentID, RoomCode, Method string
	Amount                      int64
	At                          time.Time
}

// OwnerRepo filters by the tenant of the Tx.
type OwnerRepo interface {
	Timezone(ctx context.Context, tx Tx) (string, error)
	// Revenue covers PAID payments with paid_at in [from, to), one row per building.
	Revenue(ctx context.Context, tx Tx, from, to time.Time) ([]BuildingRevenue, error)
	LatestPayments(ctx context.Context, tx Tx, limit int) ([]PaymentSummary, error)
}

// RoomCounter gives the room counts per building; *Rooms implements it.
type RoomCounter interface {
	ListBuildings(ctx context.Context, c Caller) ([]BuildingView, error)
}

// OwnerOverview is the day summary. Alerts are empty until the alert slice exists (FAST MODE).
type OwnerOverview struct {
	Date                                       string
	RevenueTotal, TransfersReceived, CashTotal int64
	ByBuilding                                 []BuildingRevenue
	OccupiedRooms, TotalRooms, OverdueRooms    int
	LatestPayments                             []PaymentSummary
}

// Owner holds the owner overview use case (SG-403).
type Owner struct {
	uow   UnitOfWork
	repo  OwnerRepo
	rooms RoomCounter
	clock Clock
	authz access.Authorizer
}

func NewOwner(uow UnitOfWork, repo OwnerRepo, rooms RoomCounter, clock Clock) *Owner {
	return &Owner{uow: uow, repo: repo, rooms: rooms, clock: clock}
}

// Overview summarises the tenant-local day of date (year, month and day are read as given); nil means today.
func (o *Owner) Overview(ctx context.Context, c Caller, date *time.Time) (OwnerOverview, error) {
	if err := o.authz.Check("getOwnerOverview", c.Role, access.EDIT); err != nil {
		return OwnerOverview{}, err
	}
	var out OwnerOverview
	err := o.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		loc, err := loadZone(ctx, tx, o.repo)
		if err != nil {
			return err
		}
		y, m, d := o.clock.Now().In(loc).Date()
		if date != nil {
			y, m, d = date.Date()
		}
		from := time.Date(y, m, d, 0, 0, 0, 0, loc)
		rows, err := o.repo.Revenue(ctx, tx, from, from.AddDate(0, 0, 1))
		if err != nil {
			return fmt.Errorf("revenue: %w", err)
		}
		latest, err := o.repo.LatestPayments(ctx, tx, latestPaymentsLimit)
		if err != nil {
			return fmt.Errorf("latest payments: %w", err)
		}
		out = OwnerOverview{Date: from.Format(time.DateOnly), ByBuilding: rows, LatestPayments: latest}
		for _, r := range rows {
			out.RevenueTotal += r.Cash + r.Transfer
			out.TransfersReceived += r.Transfer
			out.CashTotal += r.Cash
		}
		return nil
	})
	if err != nil {
		return OwnerOverview{}, err
	}
	buildings, err := o.rooms.ListBuildings(ctx, c)
	if err != nil {
		return OwnerOverview{}, fmt.Errorf("room counts: %w", err)
	}
	for _, b := range buildings {
		n := b.Counts
		out.OccupiedRooms += n.Occupied + n.Overdue
		out.OverdueRooms += n.Overdue
		out.TotalRooms += n.Vacant + n.Occupied + n.Overdue + n.ToClean + n.Maintenance
	}
	return out, nil
}
