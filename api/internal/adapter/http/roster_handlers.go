package httpadapter

import (
	"context"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// RosterService is what the roster and leave handlers need from the application layer (SG-1102, SG-1103).
type RosterService interface {
	GetRoster(ctx context.Context, c app.Caller, from, to time.Time) (app.RosterView, error)
	PutRoster(ctx context.Context, c app.Caller, key string, set, remove []app.RosterCell) (app.RosterView, error)
	CopyWeek(ctx context.Context, c app.Caller, key string, weekStart time.Time) (app.RosterView, error)
	ListLeave(ctx context.Context, c app.Caller, status string) ([]app.LeaveView, error)
	Approve(ctx context.Context, c app.Caller, key, id string) (app.LeaveView, error)
	Decline(ctx context.Context, c app.Caller, key, id, reason string) (app.LeaveView, error)
	MyRoster(ctx context.Context, c app.Caller, from, to time.Time) (app.RosterView, error)
	MyLeave(ctx context.Context, c app.Caller) (app.MyLeave, error)
	CreateLeave(ctx context.Context, c app.Caller, key string, in app.CreateLeaveInput) (app.LeaveView, error)
	CancelMine(ctx context.Context, c app.Caller, key, id string) (app.LeaveView, error)
}

// WithRoster adds the roster and leave use cases.
func (s Server) WithRoster(r RosterService) Server { s.roster = r; return s }

func date(t time.Time) openapi_types.Date { return openapi_types.Date{Time: t} }

func toLeave(l app.LeaveView) gen.LeaveRequest {
	out := gen.LeaveRequest{Id: l.ID, UserId: l.UserID, UserName: l.UserName, FromDate: date(l.From), ToDate: date(l.To), Kind: gen.LeaveKind(l.Kind),
		Status: gen.LeaveStatus(l.Status), CreatedAt: l.CreatedAt, DecidedAt: l.DecidedAt, Reason: nilIfEmpty(l.Reason),
		CoverUserId: nilIfEmpty(l.CoverUserID), DeclineReason: nilIfEmpty(l.DeclineReason)}
	if l.Shift != "" {
		code := gen.ShiftCode(l.Shift)
		out.Shift = &code
	}
	return out
}

func toRoster(v app.RosterView) gen.Roster {
	out := gen.Roster{From: date(v.From), To: date(v.To), Assignments: make([]gen.RosterAssignment, len(v.Assignments)), Leave: make([]gen.LeaveRequest, len(v.Leave))}
	for i, a := range v.Assignments {
		out.Assignments[i] = gen.RosterAssignment{UserId: a.UserID, Date: date(a.Date), Shift: gen.ShiftCode(a.Shift)}
	}
	for i, l := range v.Leave {
		out.Leave[i] = toLeave(l)
	}
	if v.Gaps != nil {
		gaps := make([]struct {
			Date  openapi_types.Date `json:"date"`
			Shift gen.ShiftCode      `json:"shift"`
		}, len(v.Gaps))
		for i, g := range v.Gaps {
			gaps[i].Date, gaps[i].Shift = date(g.Date), gen.ShiftCode(g.Shift)
		}
		out.Gaps = &gaps
	}
	return out
}

func cells(in []gen.RosterAssignment) []app.RosterCell {
	out := make([]app.RosterCell, len(in))
	for i, a := range in {
		out[i] = app.RosterCell{UserID: a.UserId, Date: a.Date.Time, Shift: string(a.Shift)}
	}
	return out
}

func (s Server) GetRoster(ctx context.Context, req gen.GetRosterRequestObject) (gen.GetRosterResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.roster.GetRoster(ctx, c, req.Params.From.Time, req.Params.To.Time)
	if err != nil {
		return nil, err
	}
	return gen.GetRoster200JSONResponse(toRoster(v)), nil
}

func (s Server) PutRoster(ctx context.Context, req gen.PutRosterRequestObject) (gen.PutRosterResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.roster.PutRoster(ctx, c, req.Params.IdempotencyKey.String(), cells(req.Body.Set), cells(req.Body.Remove))
	if err != nil {
		return nil, err
	}
	return gen.PutRoster200JSONResponse(toRoster(v)), nil
}

func (s Server) CopyRosterWeek(ctx context.Context, req gen.CopyRosterWeekRequestObject) (gen.CopyRosterWeekResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.roster.CopyWeek(ctx, c, req.Params.IdempotencyKey.String(), req.Body.WeekStart.Time)
	if err != nil {
		return nil, err
	}
	return gen.CopyRosterWeek200JSONResponse(toRoster(v)), nil
}

func (s Server) ListLeaveRequests(ctx context.Context, req gen.ListLeaveRequestsRequestObject) (gen.ListLeaveRequestsResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	status := ""
	if req.Params.Status != nil {
		status = string(*req.Params.Status)
	}
	rows, err := s.roster.ListLeave(ctx, c, status)
	if err != nil {
		return nil, err
	}
	return gen.ListLeaveRequests200JSONResponse{Items: toLeaves(rows)}, nil
}

func toLeaves(rows []app.LeaveView) []gen.LeaveRequest {
	items := make([]gen.LeaveRequest, len(rows))
	for i, l := range rows {
		items[i] = toLeave(l)
	}
	return items
}

func (s Server) ApproveLeave(ctx context.Context, req gen.ApproveLeaveRequestObject) (gen.ApproveLeaveResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	l, err := s.roster.Approve(ctx, c, req.Params.IdempotencyKey.String(), req.LeaveId)
	if err != nil {
		return nil, err
	}
	return gen.ApproveLeave200JSONResponse(toLeave(l)), nil
}

func (s Server) DeclineLeave(ctx context.Context, req gen.DeclineLeaveRequestObject) (gen.DeclineLeaveResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	l, err := s.roster.Decline(ctx, c, req.Params.IdempotencyKey.String(), req.LeaveId, req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return gen.DeclineLeave200JSONResponse(toLeave(l)), nil
}

func (s Server) GetMyRoster(ctx context.Context, req gen.GetMyRosterRequestObject) (gen.GetMyRosterResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	v, err := s.roster.MyRoster(ctx, c, req.Params.From.Time, req.Params.To.Time)
	if err != nil {
		return nil, err
	}
	return gen.GetMyRoster200JSONResponse(toRoster(v)), nil
}

func (s Server) ListMyLeaveRequests(ctx context.Context, _ gen.ListMyLeaveRequestsRequestObject) (gen.ListMyLeaveRequestsResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	m, err := s.roster.MyLeave(ctx, c)
	if err != nil {
		return nil, err
	}
	return gen.ListMyLeaveRequests200JSONResponse{Items: toLeaves(m.Items),
		Balance: gen.LeaveBalance{Year: m.Balance.Year, Annual: m.Balance.Annual, Used: m.Balance.Used, Left: m.Balance.Left}}, nil
}

func (s Server) CreateLeaveRequest(ctx context.Context, req gen.CreateLeaveRequestRequestObject) (gen.CreateLeaveRequestResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := app.CreateLeaveInput{From: b.FromDate.Time, To: b.ToDate.Time, Kind: string(b.Kind), Reason: deref(b.Reason), CoverID: deref(b.CoverUserId)}
	if b.Shift != nil {
		in.Shift = string(*b.Shift)
	}
	l, err := s.roster.CreateLeave(ctx, c, req.Params.IdempotencyKey.String(), in)
	if err != nil {
		return nil, err
	}
	return gen.CreateLeaveRequest201JSONResponse(toLeave(l)), nil
}

func (s Server) CancelMyLeave(ctx context.Context, req gen.CancelMyLeaveRequestObject) (gen.CancelMyLeaveResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	l, err := s.roster.CancelMine(ctx, c, req.Params.IdempotencyKey.String(), req.LeaveId)
	if err != nil {
		return nil, err
	}
	return gen.CancelMyLeave200JSONResponse(toLeave(l)), nil
}
