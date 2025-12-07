package hostlibrary

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/plugin/sdk/nodes"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type NodesServiceImpl struct {
	nodeRepo repositories.NodeRepository
}

func NewNodesService(nodeRepo repositories.NodeRepository) *NodesServiceImpl {
	return &NodesServiceImpl{
		nodeRepo: nodeRepo,
	}
}

func (s *NodesServiceImpl) FindNodes(
	ctx context.Context,
	req *nodes.FindNodesRequest,
) (*nodes.FindNodesResponse, error) {
	var filter *filters.FindNode
	if req.Filter != nil {
		filter = &filters.FindNode{
			IDs: uintsFromUint64s(req.Filter.Ids),
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

	result, err := s.nodeRepo.Find(ctx, filter, sorting, pagination)
	if err != nil {
		return nil, err
	}

	return &nodes.FindNodesResponse{
		Nodes: convertNodesToProto(result),
		Total: int32(len(result)), //nolint:gosec
	}, nil
}

func (s *NodesServiceImpl) GetNode(
	ctx context.Context,
	req *nodes.GetNodeRequest,
) (*nodes.GetNodeResponse, error) {
	result, err := s.nodeRepo.Find(ctx, filters.FindNodeByIDs(uint(req.Id)), nil, nil)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return &nodes.GetNodeResponse{Found: false}, nil
	}

	return &nodes.GetNodeResponse{
		Node:  convertNodeToProto(&result[0]),
		Found: true,
	}, nil
}

func convertNodesToProto(nds []domain.Node) []*proto.Node {
	return lo.Map(nds, func(n domain.Node, _ int) *proto.Node {
		return convertNodeToProto(&n)
	})
}

func convertNodeToProto(n *domain.Node) *proto.Node {
	return &proto.Node{
		Id:          uint64(n.ID),
		Name:        n.Name,
		Enabled:     n.Enabled,
		Os:          string(n.OS),
		Location:    n.Location,
		Provider:    n.Provider,
		Ips:         n.IPs,
		WorkPath:    n.WorkPath,
		GdaemonHost: n.GdaemonHost,
		GdaemonPort: int32(n.GdaemonPort), //nolint:gosec
	}
}

type NodesHostLibrary struct {
	impl *NodesServiceImpl
}

func NewNodesHostLibrary(nodeRepo repositories.NodeRepository) *NodesHostLibrary {
	return &NodesHostLibrary{
		impl: NewNodesService(nodeRepo),
	}
}

func (l *NodesHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return nodes.Instantiate(ctx, r, l.impl)
}
