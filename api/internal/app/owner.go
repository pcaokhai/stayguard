package app

import (
	"context"
	"fmt"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/access"
	"github.com/pcaokhai/stayguard/api/internal/domain/room"
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

// RoomCounter gives the room counts per building and the rooms of one; *Rooms implements it.
type RoomCounter interface {
	ListBuildings(ctx context.Context, c Caller) ([]BuildingView, error)
	ListRooms(ctx context.Context, c Caller, buildingID string, status *room.Status) ([]RoomView, error)
}

const (
	overviewAlertsLimit = 5
	// longToCleanMinutes is how long a room may wait to be cleaned before the owner is told.
	longToCleanMinutes = 60
)

// BuildingStatus is the room counts and the day's revenue of one building. Occupied includes overdue rooms.
type BuildingStatus struct {
	ID, Code                                               string
	Total, Occupied, Vacant, ToClean, Overdue, Maintenance int
	OccupancyPct                                           float64
	RevenueToday                                           int64
}

// AttentionItem is something that needs the owner now. Ref is an alert id or a room code.
type AttentionItem struct {
	Kind, Ref, RoomCode string
	Minutes             *int
	Amount              *int64
}

// OwnerOverview is the day summary. Alerts are empty until the alert slice exists (FAST MODE).
type OwnerOverview struct {
	Date                                       string
	RevenueTotal, TransfersReceived, CashTotal int64
	ByBuilding                                 []BuildingRevenue
	OccupiedRooms, TotalRooms, OverdueRooms    int
	LatestPayments                             []PaymentSummary
	Buildings                                  []BuildingStatus
	Alerts                                     []AlertRow
	Attention                                  []AttentionItem
}

// Owner holds the owner overview use case (SG-403).
type Owner struct {
	uow   UnitOfWork
	repo  OwnerRepo
	rooms RoomCounter
	clock Clock
	// monitor is optional (nil leaves alerts and attention empty): the unread alerts and the rooms waiting too long.
	monitor MonitorRepo
	authz   access.Authorizer
}

// WithMonitor adds the alert and cleaning data to the overview.
func (o *Owner) WithMonitor(m MonitorRepo) *Owner { o.monitor = m; return o }

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
		return o.watch(ctx, tx, &out)
	})
	if err != nil {
		return OwnerOverview{}, err
	}
	buildings, err := o.rooms.ListBuildings(ctx, c)
	if err != nil {
		return OwnerOverview{}, fmt.Errorf("room counts: %w", err)
	}
	revenue := map[string]int64{}
	for _, r := range out.ByBuilding {
		revenue[r.BuildingID] = r.Cash + r.Transfer
	}
	out.Buildings = make([]BuildingStatus, 0, len(buildings))
	for _, b := range buildings {
		n := b.Counts
		total := n.Vacant + n.Occupied + n.Overdue + n.ToClean + n.Maintenance
		out.OccupiedRooms += n.Occupied + n.Overdue
		out.OverdueRooms += n.Overdue
		out.TotalRooms += total
		out.Buildings = append(out.Buildings, BuildingStatus{ID: b.ID, Code: b.Code, Total: total, Occupied: n.Occupied + n.Overdue,
			Vacant: n.Vacant, ToClean: n.ToClean, Overdue: n.Overdue, Maintenance: n.Maintenance,
			OccupancyPct: occupancyPct(n.Occupied+n.Overdue, total), RevenueToday: revenue[b.ID]})
		if n.Overdue > 0 {
			if err := o.overdueRooms(ctx, c, b.ID, &out); err != nil {
				return OwnerOverview{}, err
			}
		}
	}
	return out, nil
}

// occupancyPct is a percentage with one decimal.
func occupancyPct(occupied, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(occupied*1000/total) / 10
}

// overdueRooms adds one OVERDUE_ROOM item per overdue room of a building.
// ponytail: no minutes overdue (the room map derives the end of a stay from its snapshot); add when the web board shows it.
func (o *Owner) overdueRooms(ctx context.Context, c Caller, buildingID string, out *OwnerOverview) error {
	st := room.StatusOverdue
	rooms, err := o.rooms.ListRooms(ctx, c, buildingID, &st)
	if err != nil {
		return fmt.Errorf("overdue rooms: %w", err)
	}
	for _, r := range rooms {
		out.Attention = append(out.Attention, AttentionItem{Kind: "OVERDUE_ROOM", Ref: r.ID, RoomCode: r.Code})
	}
	return nil
}

// watch fills the unread alerts and the attention items that come from alerts and from rooms left uncleaned.
func (o *Owner) watch(ctx context.Context, tx Tx, out *OwnerOverview) error {
	out.Alerts, out.Attention = []AlertRow{}, []AttentionItem{}
	if o.monitor == nil {
		return nil
	}
	now := storedTime(o.clock.Now())
	alerts, err := o.monitor.Alerts(ctx, tx, AlertFilter{UnreadOnly: true, Limit: overviewAlertsLimit})
	if err != nil {
		return fmt.Errorf("unread alerts: %w", err)
	}
	out.Alerts = alerts
	for _, a := range alerts {
		switch a.Kind {
		case AlertPaymentMismatch, AlertUnmatchedTransfer, AlertCashShort, AlertOverpaid, AlertPaymentPartial, AlertPaymentUnpaid:
			out.Attention = append(out.Attention, AttentionItem{Kind: a.Kind, Ref: a.ID, RoomCode: a.RoomCode, Amount: a.Amount})
		}
	}
	waiting, err := o.monitor.LongToClean(ctx, tx, now.Add(-longToCleanMinutes*time.Minute))
	if err != nil {
		return fmt.Errorf("long to clean: %w", err)
	}
	for _, w := range waiting {
		minutes := int(now.Sub(w.Since) / time.Minute)
		out.Attention = append(out.Attention, AttentionItem{Kind: "LONG_TO_CLEAN", Ref: w.RoomCode, RoomCode: w.RoomCode, Minutes: &minutes})
	}
	tickets, err := o.monitor.OpenTickets(ctx, tx)
	if err != nil {
		return fmt.Errorf("open tickets: %w", err)
	}
	for _, t := range tickets {
		out.Attention = append(out.Attention, AttentionItem{Kind: "TICKET_OPEN", Ref: t.ID, RoomCode: t.RoomCode})
	}
	leave, err := o.monitor.PendingLeave(ctx, tx)
	if err != nil {
		return fmt.Errorf("pending leave: %w", err)
	}
	for _, l := range leave {
		out.Attention = append(out.Attention, AttentionItem{Kind: "LEAVE_PENDING", Ref: l.ID})
	}
	return nil
}
