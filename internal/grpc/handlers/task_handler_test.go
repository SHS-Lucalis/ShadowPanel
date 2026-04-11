package handlers

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDaemonTaskRepo(t *testing.T, tasks ...*domain.DaemonTask) *inmemory.DaemonTaskRepository {
	t.Helper()

	repo := inmemory.NewDaemonTaskRepository()
	for _, task := range tasks {
		require.NoError(t, repo.Save(context.Background(), task))
	}

	return repo
}

func newTestServerForTask(installed domain.ServerInstalledStatus) *domain.Server {
	return &domain.Server{
		ID: 1, UUID: uuid.New(), UUIDShort: "s1",
		Name: "Server", GameID: "cs", DSID: 1, GameModID: 1,
		ServerIP: "127.0.0.1", ServerPort: 27015, Dir: "/srv/s",
		Installed: installed,
	}
}

func TestHandleTaskStatusUpdate_ServerInstalledStatus(t *testing.T) {
	now := time.Now()
	serverID := uint(1)

	tests := []struct {
		name              string
		task              *domain.DaemonTask
		server            *domain.Server
		updateStatus      proto.DaemonTaskStatus
		wantInstalled     domain.ServerInstalledStatus
		wantServerUpdated bool
	}{
		{
			name: "gsinst_success_sets_installed",
			task: &domain.DaemonTask{
				DedicatedServerID: 1, ServerID: &serverID,
				Task: domain.DaemonTaskTypeServerInstall, Status: domain.DaemonTaskStatusWorking,
				CreatedAt: &now, UpdatedAt: &now,
			},
			server:            newTestServerForTask(domain.ServerInstalledStatusNotInstalled),
			updateStatus:      proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS,
			wantInstalled:     domain.ServerInstalledStatusInstalled,
			wantServerUpdated: true,
		},
		{
			name: "gsinst_working_sets_installation_in_progress",
			task: &domain.DaemonTask{
				DedicatedServerID: 1, ServerID: &serverID,
				Task: domain.DaemonTaskTypeServerInstall, Status: domain.DaemonTaskStatusWaiting,
				CreatedAt: &now, UpdatedAt: &now,
			},
			server:            newTestServerForTask(domain.ServerInstalledStatusNotInstalled),
			updateStatus:      proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WORKING,
			wantInstalled:     domain.ServerInstalledStatusInstallationInProg,
			wantServerUpdated: true,
		},
		{
			name: "gsinst_error_sets_not_installed",
			task: &domain.DaemonTask{
				DedicatedServerID: 1, ServerID: &serverID,
				Task: domain.DaemonTaskTypeServerInstall, Status: domain.DaemonTaskStatusWorking,
				CreatedAt: &now, UpdatedAt: &now,
			},
			server:            newTestServerForTask(domain.ServerInstalledStatusInstallationInProg),
			updateStatus:      proto.DaemonTaskStatus_DAEMON_TASK_STATUS_ERROR,
			wantInstalled:     domain.ServerInstalledStatusNotInstalled,
			wantServerUpdated: true,
		},
		{
			name: "gsinst_canceled_sets_not_installed",
			task: &domain.DaemonTask{
				DedicatedServerID: 1, ServerID: &serverID,
				Task: domain.DaemonTaskTypeServerInstall, Status: domain.DaemonTaskStatusWorking,
				CreatedAt: &now, UpdatedAt: &now,
			},
			server:            newTestServerForTask(domain.ServerInstalledStatusInstallationInProg),
			updateStatus:      proto.DaemonTaskStatus_DAEMON_TASK_STATUS_CANCELED,
			wantInstalled:     domain.ServerInstalledStatusNotInstalled,
			wantServerUpdated: true,
		},
		{
			name: "gsdel_success_sets_not_installed",
			task: &domain.DaemonTask{
				DedicatedServerID: 1, ServerID: &serverID,
				Task: domain.DaemonTaskTypeServerDelete, Status: domain.DaemonTaskStatusWorking,
				CreatedAt: &now, UpdatedAt: &now,
			},
			server:            newTestServerForTask(domain.ServerInstalledStatusInstalled),
			updateStatus:      proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS,
			wantInstalled:     domain.ServerInstalledStatusNotInstalled,
			wantServerUpdated: true,
		},
		{
			name: "gsstart_success_does_not_change_installed",
			task: &domain.DaemonTask{
				DedicatedServerID: 1, ServerID: &serverID,
				Task: domain.DaemonTaskTypeServerStart, Status: domain.DaemonTaskStatusWorking,
				CreatedAt: &now, UpdatedAt: &now,
			},
			server:            newTestServerForTask(domain.ServerInstalledStatusInstalled),
			updateStatus:      proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS,
			wantInstalled:     domain.ServerInstalledStatusInstalled,
			wantServerUpdated: false,
		},
		{
			name: "task_without_server_id_skips_update",
			task: &domain.DaemonTask{
				DedicatedServerID: 1, ServerID: nil,
				Task: domain.DaemonTaskTypeServerInstall, Status: domain.DaemonTaskStatusWorking,
				CreatedAt: &now, UpdatedAt: &now,
			},
			server:            nil,
			updateStatus:      proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS,
			wantServerUpdated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskRepo := setupDaemonTaskRepo(t, tt.task)
			serverRepo := inmemory.NewServerRepository()

			if tt.server != nil {
				require.NoError(t, serverRepo.Save(context.Background(), tt.server))
			}

			handler := NewTaskHandler(taskRepo, serverRepo, nil, slog.Default())

			err := handler.HandleTaskStatusUpdate(context.Background(), 1, &proto.TaskStatusUpdate{
				TaskId: uint64(tt.task.ID),
				Status: tt.updateStatus,
			})
			require.NoError(t, err)

			if !tt.wantServerUpdated || tt.server == nil {
				return
			}

			servers, err := serverRepo.Find(
				context.Background(),
				&filters.FindServer{IDs: []uint{tt.server.ID}},
				nil, nil,
			)
			require.NoError(t, err)
			require.Len(t, servers, 1)
			assert.Equal(t, tt.wantInstalled, servers[0].Installed)
		})
	}
}

func TestResolveInstalledStatus(t *testing.T) {
	tests := []struct {
		name       string
		taskType   domain.DaemonTaskType
		taskStatus domain.DaemonTaskStatus
		want       domain.ServerInstalledStatus
		wantOK     bool
	}{
		{
			name:       "gsinst_waiting_no_change",
			taskType:   domain.DaemonTaskTypeServerInstall,
			taskStatus: domain.DaemonTaskStatusWaiting,
			wantOK:     false,
		},
		{
			name:       "gsdel_working_no_change",
			taskType:   domain.DaemonTaskTypeServerDelete,
			taskStatus: domain.DaemonTaskStatusWorking,
			wantOK:     false,
		},
		{
			name:       "gsdel_error_no_change",
			taskType:   domain.DaemonTaskTypeServerDelete,
			taskStatus: domain.DaemonTaskStatusError,
			wantOK:     false,
		},
		{
			name:       "gsupd_success_no_change",
			taskType:   domain.DaemonTaskTypeServerUpdate,
			taskStatus: domain.DaemonTaskStatusSuccess,
			wantOK:     false,
		},
		{
			name:       "gsinst_success",
			taskType:   domain.DaemonTaskTypeServerInstall,
			taskStatus: domain.DaemonTaskStatusSuccess,
			want:       domain.ServerInstalledStatusInstalled,
			wantOK:     true,
		},
		{
			name:       "gsinst_working",
			taskType:   domain.DaemonTaskTypeServerInstall,
			taskStatus: domain.DaemonTaskStatusWorking,
			want:       domain.ServerInstalledStatusInstallationInProg,
			wantOK:     true,
		},
		{
			name:       "gsdel_success",
			taskType:   domain.DaemonTaskTypeServerDelete,
			taskStatus: domain.DaemonTaskStatusSuccess,
			want:       domain.ServerInstalledStatusNotInstalled,
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveInstalledStatus(tt.taskType, tt.taskStatus)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
