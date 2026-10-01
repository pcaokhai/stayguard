package httpadapter

import (
	"context"

	"github.com/pcaokhai/stayguard/api/internal/adapter/http/gen"
	"github.com/pcaokhai/stayguard/api/internal/app"
)

// HousekeepingService is what the housekeeping handlers need from the application layer.
type HousekeepingService interface {
	ListTasks(ctx context.Context, c app.Caller) ([]app.HousekeepingTask, error)
	Complete(ctx context.Context, c app.Caller, taskID, retryID string) (app.HousekeepingTask, error)
}

func (s Server) ListHousekeepingTasks(ctx context.Context, _ gen.ListHousekeepingTasksRequestObject) (gen.ListHousekeepingTasksResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	tasks, err := s.housekeeping.ListTasks(ctx, c)
	if err != nil {
		return nil, err
	}
	items := make([]gen.HousekeepingTask, len(tasks))
	for i, t := range tasks {
		items[i] = toTask(t)
	}
	return gen.ListHousekeepingTasks200JSONResponse{Items: items}, nil
}

func (s Server) CompleteHousekeepingTask(ctx context.Context, req gen.CompleteHousekeepingTaskRequestObject) (gen.CompleteHousekeepingTaskResponseObject, error) {
	c, err := s.featureCaller(ctx, true)
	if err != nil {
		return nil, err
	}
	t, err := s.housekeeping.Complete(ctx, c, req.TaskId, req.Params.IdempotencyKey.String())
	if err != nil {
		return nil, err
	}
	return gen.CompleteHousekeepingTask200JSONResponse(toTask(t)), nil
}

func toTask(t app.HousekeepingTask) gen.HousekeepingTask {
	status := gen.HousekeepingTaskStatusOPEN
	if t.Done {
		status = gen.HousekeepingTaskStatusDONE
	}
	return gen.HousekeepingTask{Id: t.ID, RoomId: t.ID, RoomCode: t.RoomCode, BuildingId: t.BuildingID, Status: status,
		CreatedAt: t.CreatedAt, CompletedAt: t.CompletedAt}
}
