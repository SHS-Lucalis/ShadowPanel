package hostlibrary

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/plugin/sdk/daemontasks"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type DaemonTasksServiceImpl struct {
	daemonTaskRepo repositories.DaemonTaskRepository
}

func NewDaemonTasksService(daemonTaskRepo repositories.DaemonTaskRepository) *DaemonTasksServiceImpl {
	return &DaemonTasksServiceImpl{
		daemonTaskRepo: daemonTaskRepo,
	}
}

func (s *DaemonTasksServiceImpl) FindDaemonTasks(
	ctx context.Context,
	req *daemontasks.FindDaemonTasksRequest,
) (*daemontasks.FindDaemonTasksResponse, error) {
	var filter *filters.FindDaemonTask
	if req.Filter != nil {
		filter = &filters.FindDaemonTask{
			IDs:                uintsFromUint64s(req.Filter.Ids),
			DedicatedServerIDs: uintsFromUint64s(req.Filter.NodeIds),
			ServerIDs:          uintPtrsFromUint64s(req.Filter.ServerIds),
		}
	}

	var pagination *filters.Pagination
	if req.Pagination != nil {
		pagination = &filters.Pagination{
			Limit:  int(req.Pagination.Limit),
			Offset: int(req.Pagination.Offset),
		}
	}

	sorting := convertSorting(req.Sorting)

	tasks, err := s.daemonTaskRepo.Find(ctx, filter, sorting, pagination)
	if err != nil {
		return nil, err
	}

	return &daemontasks.FindDaemonTasksResponse{
		Tasks: convertDaemonTasksToProto(tasks),
		Total: int32(len(tasks)), //nolint:gosec
	}, nil
}

func (s *DaemonTasksServiceImpl) CreateDaemonTask(
	ctx context.Context,
	req *daemontasks.CreateDaemonTaskRequest,
) (*daemontasks.CreateDaemonTaskResponse, error) {
	task := &domain.DaemonTask{
		DedicatedServerID: uint(req.NodeId),
		Task:              domain.DaemonTaskType(req.TaskType),
		Status:            domain.DaemonTaskStatusWaiting,
	}

	if req.ServerId != nil {
		serverID := uint(*req.ServerId)
		task.ServerID = &serverID
	}

	if req.RunAfterId != nil {
		runAfterID := uint(*req.RunAfterId)
		task.RunAftID = &runAfterID
	}

	if req.Cmd != nil {
		task.Cmd = req.Cmd
	}

	if err := s.daemonTaskRepo.Save(ctx, task); err != nil {
		return &daemontasks.CreateDaemonTaskResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &daemontasks.CreateDaemonTaskResponse{
		Success: true,
		TaskId:  uint64(task.ID),
	}, nil
}

func convertDaemonTasksToProto(tasks []domain.DaemonTask) []*proto.DaemonTask {
	return lo.Map(tasks, func(t domain.DaemonTask, _ int) *proto.DaemonTask {
		return convertDaemonTaskToProto(&t)
	})
}

func convertDaemonTaskToProto(t *domain.DaemonTask) *proto.DaemonTask {
	var serverID, runAfterID *uint64
	if t.ServerID != nil {
		serverID = lo.ToPtr(uint64(*t.ServerID))
	}
	if t.RunAftID != nil {
		runAfterID = lo.ToPtr(uint64(*t.RunAftID))
	}

	return &proto.DaemonTask{
		Id:         uint64(t.ID),
		NodeId:     uint64(t.DedicatedServerID),
		ServerId:   serverID,
		RunAfterId: runAfterID,
		TaskType:   string(t.Task),
		Cmd:        t.Cmd,
		Output:     t.Output,
		Status:     string(t.Status),
	}
}

type DaemonTasksHostLibrary struct {
	impl *DaemonTasksServiceImpl
}

func NewDaemonTasksHostLibrary(daemonTaskRepo repositories.DaemonTaskRepository) *DaemonTasksHostLibrary {
	return &DaemonTasksHostLibrary{
		impl: NewDaemonTasksService(daemonTaskRepo),
	}
}

func (l *DaemonTasksHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return daemontasks.Instantiate(ctx, r, l.impl)
}
