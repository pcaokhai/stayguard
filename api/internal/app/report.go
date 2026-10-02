package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pcaokhai/stayguard/api/internal/domain/finance"
	"github.com/pcaokhai/stayguard/api/internal/domain/stay"
)

// MonthTotals is one month of the report.
type MonthTotals struct {
	Month             string
	Revenue, Expenses int64
}

// IncomeCostReport is revenue, expenses and profit for a month range (SG-1203). Revenue is what paid invoices were
// for, by the day they were paid; the four revenue splits each add up to it.
type IncomeCostReport struct {
	From, To                                                                    string
	Revenue, Expenses, Profit                                                   int64
	MarginPct, OccupancyPct                                                     float64
	Months                                                                      []MonthTotals
	ExpensesByCategory, RevenueByRentalType, RevenueByBuilding, RevenueByMethod []Share
}

// Reports builds the income and cost report.
type Reports struct {
	uow      UnitOfWork
	repo     ReportRepo
	expenses ExpenseRepo
	clock    Clock
	guard
}

func NewReports(uow UnitOfWork, repo ReportRepo, expenses ExpenseRepo, levels BuildingLevels, clock Clock) *Reports {
	return &Reports{uow: uow, repo: repo, expenses: expenses, clock: clock, guard: guard{levels: levels}}
}

// IncomeCost reports the months from..to (YYYY-MM, both included, at most 36).
func (r *Reports) IncomeCost(ctx context.Context, c Caller, fromMonth, toMonth string) (IncomeCostReport, error) {
	if err := r.checkRole("getIncomeCostReport", c); err != nil {
		return IncomeCostReport{}, err
	}
	months, err := finance.MonthsBetween(fromMonth, toMonth)
	if err != nil {
		return IncomeCostReport{}, stay.NewValidationError([]stay.FieldError{{Path: "to", Code: stay.CodePattern}})
	}
	var out IncomeCostReport
	err = r.uow.Do(ctx, c.TenantID, func(ctx context.Context, tx Tx) error {
		cur, err := currentMonth(ctx, tx, r.repo, r.clock)
		if err != nil {
			return err
		}
		if _, err := EnsureRecurring(ctx, tx, r.expenses, min(toMonth, cur)); err != nil {
			return err
		}
		out, err = r.build(ctx, tx, months)
		return err
	})
	return out, err
}

func (r *Reports) build(ctx context.Context, tx Tx, months []string) (IncomeCostReport, error) {
	zone, err := r.repo.Timezone(ctx, tx)
	if err != nil {
		return IncomeCostReport{}, fmt.Errorf("timezone: %w", err)
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return IncomeCostReport{}, fmt.Errorf("tenant timezone: %w", err)
	}
	first, last := months[0], months[len(months)-1]
	f, _, _ := finance.MonthDays(first)
	l, lastDay, _ := finance.MonthDays(last)
	from := time.Date(f.Year(), f.Month(), 1, 0, 0, 0, 0, loc)
	to := time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0)

	rep := IncomeCostReport{From: first, To: last}
	if rep.Revenue, err = r.repo.Revenue(ctx, tx, from, to); err != nil {
		return IncomeCostReport{}, fmt.Errorf("revenue: %w", err)
	}
	if rep.RevenueByRentalType, err = r.repo.RevenueByRentalType(ctx, tx, from, to); err != nil {
		return IncomeCostReport{}, fmt.Errorf("revenue by rental type: %w", err)
	}
	if rep.RevenueByBuilding, err = r.repo.RevenueByBuilding(ctx, tx, from, to); err != nil {
		return IncomeCostReport{}, fmt.Errorf("revenue by building: %w", err)
	}
	if rep.RevenueByMethod, err = r.byMethod(ctx, tx, from, to); err != nil {
		return IncomeCostReport{}, err
	}
	if rep.ExpensesByCategory, err = r.repo.ExpenseByCategory(ctx, tx, first, last); err != nil {
		return IncomeCostReport{}, fmt.Errorf("expenses by category: %w", err)
	}
	for _, s := range rep.ExpensesByCategory {
		rep.Expenses += s.Amount
	}
	if rep.Months, err = r.monthRows(ctx, tx, months, from, to, zone); err != nil {
		return IncomeCostReport{}, err
	}
	rep.Profit = rep.Revenue - rep.Expenses
	rep.MarginPct = finance.MarginPct(rep.Revenue, rep.Expenses)
	rep.OccupancyPct, err = r.occupancy(ctx, tx, zone, f, lastDay)
	return rep, err
}

// byMethod splits revenue by how it was paid: the deposit (taken in cash at check-in) plus the rest by payment method.
func (r *Reports) byMethod(ctx context.Context, tx Tx, from, to time.Time) ([]Share, error) {
	deposit, err := r.repo.RevenueDeposit(ctx, tx, from, to)
	if err != nil {
		return nil, fmt.Errorf("revenue deposit part: %w", err)
	}
	rest, err := r.repo.RevenueBalanceByMethod(ctx, tx, from, to)
	if err != nil {
		return nil, fmt.Errorf("revenue by method: %w", err)
	}
	sum := map[string]int64{}
	if deposit > 0 {
		sum["CASH"] += deposit
	}
	for _, s := range rest {
		sum[s.Key] += s.Amount
	}
	out := make([]Share, 0, len(sum))
	for k, v := range sum {
		out = append(out, Share{Key: k, Amount: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (r *Reports) monthRows(ctx context.Context, tx Tx, months []string, from, to time.Time, zone string) ([]MonthTotals, error) {
	rev, err := r.repo.RevenueByMonth(ctx, tx, from, to, zone)
	if err != nil {
		return nil, fmt.Errorf("revenue by month: %w", err)
	}
	exp, err := r.repo.ExpenseByMonth(ctx, tx, months[0], months[len(months)-1])
	if err != nil {
		return nil, fmt.Errorf("expenses by month: %w", err)
	}
	out := make([]MonthTotals, len(months))
	for i, m := range months {
		out[i] = MonthTotals{Month: m, Revenue: rev[m], Expenses: exp[m]}
	}
	return out, nil
}

// occupancy is occupied room-days over available room-days of the range; the end of the range is cut at today.
func (r *Reports) occupancy(ctx context.Context, tx Tx, zone string, first, last time.Time) (float64, error) {
	loc, _ := time.LoadLocation(zone) // checked by the caller
	today := calendarDay(r.clock.Now().In(loc))
	if last.After(today) {
		last = today
	}
	if last.Before(first) {
		return 0, nil
	}
	rooms, err := r.repo.RoomCount(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("room count: %w", err)
	}
	days, err := r.repo.OccupiedRoomDays(ctx, tx, zone, first, last)
	if err != nil {
		return 0, fmt.Errorf("occupied room days: %w", err)
	}
	return finance.OccupancyPct(days, rooms*int64(last.Sub(first)/(24*time.Hour)+1)), nil
}
