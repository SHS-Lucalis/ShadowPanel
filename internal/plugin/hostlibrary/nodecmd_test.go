package hostlibrary

import (
	"context"
	"testing"

	"github.com/gameap/gameap/internal/daemon"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/pkg/plugin/sdk/nodecmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCommandService struct {
	executeFunc func(ctx context.Context, node *domain.Node, command string, opts ...daemon.CommandServiceOption) (*daemon.CommandResult, error)
}

func (m *mockCommandService) ExecuteCommand(
	ctx context.Context,
	node *domain.Node,
	command string,
	opts ...daemon.CommandServiceOption,
) (*daemon.CommandResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, node, command, opts...)
	}

	return &daemon.CommandResult{}, nil
}

type nodeCmdServiceImplForTest struct {
	commandService *mockCommandService
	nodeRepo       *inmemory.NodeRepository
}

func newNodeCmdServiceForTest(
	commandService *mockCommandService,
	nodeRepo *inmemory.NodeRepository,
) *nodeCmdServiceImplForTest {
	return &nodeCmdServiceImplForTest{
		commandService: commandService,
		nodeRepo:       nodeRepo,
	}
}

func (s *nodeCmdServiceImplForTest) getNode(ctx context.Context, nodeID uint64) (*domain.Node, error) {
	nodes, err := s.nodeRepo.Find(ctx, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	for i := range nodes {
		if nodes[i].ID == uint(nodeID) {
			return &nodes[i], nil
		}
	}

	return nil, nil
}

func (s *nodeCmdServiceImplForTest) ExecuteCommand(
	ctx context.Context,
	req *nodecmd.ExecuteCommandRequest,
) (*nodecmd.ExecuteCommandResponse, error) {
	node, err := s.getNode(ctx, req.NodeId)
	if err != nil {
		return &nodecmd.ExecuteCommandResponse{Error: new(err.Error())}, nil
	}

	if node == nil {
		return &nodecmd.ExecuteCommandResponse{Error: new("node not found")}, nil
	}

	var opts []daemon.CommandServiceOption
	if req.WorkDir != nil {
		opts = append(opts, daemon.CommandServiceOptionWithWorkDir(*req.WorkDir))
	}

	result, err := s.commandService.ExecuteCommand(ctx, node, req.Command, opts...)
	if err != nil {
		return &nodecmd.ExecuteCommandResponse{Error: new(err.Error())}, nil
	}

	return &nodecmd.ExecuteCommandResponse{
		Output:   result.Output,
		ExitCode: int32(result.ExitCode),
	}, nil
}

//go:fix inline
func ptrString(s string) *string {
	return new(s)
}

func TestNodeCmdService_ExecuteCommand(t *testing.T) {
	tests := []struct {
		name         string
		setupRepo    func(*inmemory.NodeRepository)
		setupCmd     func() *mockCommandService
		request      *nodecmd.ExecuteCommandRequest
		wantError    string
		wantOutput   string
		wantExitCode int32
	}{
		{
			name:      "node_not_found_returns_error",
			setupRepo: func(_ *inmemory.NodeRepository) {},
			setupCmd: func() *mockCommandService {
				return &mockCommandService{}
			},
			request: &nodecmd.ExecuteCommandRequest{
				NodeId:  999,
				Command: "echo hello",
			},
			wantError: "node not found",
		},
		{
			name: "command_executed_successfully",
			setupRepo: func(r *inmemory.NodeRepository) {
				_ = r.Save(context.Background(), &domain.Node{
					Name: "TestNode",
					OS:   domain.NodeOSLinux,
				})
			},
			setupCmd: func() *mockCommandService {
				return &mockCommandService{
					executeFunc: func(_ context.Context, _ *domain.Node, _ string, _ ...daemon.CommandServiceOption) (*daemon.CommandResult, error) {
						return &daemon.CommandResult{
							Output:   "hello\n",
							ExitCode: 0,
						}, nil
					},
				}
			},
			request: &nodecmd.ExecuteCommandRequest{
				NodeId:  1,
				Command: "echo hello",
			},
			wantOutput:   "hello\n",
			wantExitCode: 0,
		},
		{
			name: "command_with_workdir",
			setupRepo: func(r *inmemory.NodeRepository) {
				_ = r.Save(context.Background(), &domain.Node{
					Name: "TestNode",
					OS:   domain.NodeOSLinux,
				})
			},
			setupCmd: func() *mockCommandService {
				return &mockCommandService{
					executeFunc: func(_ context.Context, _ *domain.Node, _ string, _ ...daemon.CommandServiceOption) (*daemon.CommandResult, error) {
						return &daemon.CommandResult{
							Output:   "/home/user\n",
							ExitCode: 0,
						}, nil
					},
				}
			},
			request: &nodecmd.ExecuteCommandRequest{
				NodeId:  1,
				Command: "pwd",
				WorkDir: new("/home/user"),
			},
			wantOutput:   "/home/user\n",
			wantExitCode: 0,
		},
		{
			name: "command_returns_nonzero_exit_code",
			setupRepo: func(r *inmemory.NodeRepository) {
				_ = r.Save(context.Background(), &domain.Node{
					Name: "TestNode",
					OS:   domain.NodeOSLinux,
				})
			},
			setupCmd: func() *mockCommandService {
				return &mockCommandService{
					executeFunc: func(_ context.Context, _ *domain.Node, _ string, _ ...daemon.CommandServiceOption) (*daemon.CommandResult, error) {
						return &daemon.CommandResult{
							Output:   "command not found",
							ExitCode: 127,
						}, nil
					},
				}
			},
			request: &nodecmd.ExecuteCommandRequest{
				NodeId:  1,
				Command: "nonexistent_command",
			},
			wantOutput:   "command not found",
			wantExitCode: 127,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := inmemory.NewNodeRepository()
			tt.setupRepo(repo)
			cmdSvc := tt.setupCmd()

			svc := newNodeCmdServiceForTest(cmdSvc, repo)
			resp, err := svc.ExecuteCommand(context.Background(), tt.request)

			require.NoError(t, err)

			if tt.wantError != "" {
				require.NotNil(t, resp.Error)
				assert.Contains(t, *resp.Error, tt.wantError)

				return
			}

			assert.Nil(t, resp.Error)
			assert.Equal(t, tt.wantOutput, resp.Output)
			assert.Equal(t, tt.wantExitCode, resp.ExitCode)
		})
	}
}

func TestNewNodeCmdHostLibrary(t *testing.T) {
	repo := inmemory.NewNodeRepository()
	lib := NewNodeCmdHostLibrary(nil, repo)

	assert.NotNil(t, lib)
	assert.NotNil(t, lib.impl)
}
