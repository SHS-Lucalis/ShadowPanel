package uploadsession

import (
	"context"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	serversbase "github.com/gameap/gameap/internal/api/servers/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/api"
	"github.com/pkg/errors"
)

type Resolver struct {
	finder   *serversbase.ServerFinder
	checker  *serversbase.AbilityChecker
	nodeRepo repositories.NodeRepository
}

func NewResolver(
	serverRepo repositories.ServerRepository,
	nodeRepo repositories.NodeRepository,
	rbac base.RBAC,
) *Resolver {
	return &Resolver{
		finder:   serversbase.NewServerFinder(serverRepo, rbac),
		checker:  serversbase.NewAbilityChecker(rbac),
		nodeRepo: nodeRepo,
	}
}

type ServerNode struct {
	Server *domain.Server
	Node   *domain.Node
}

func (r *Resolver) Resolve(
	ctx context.Context,
	user *domain.User,
	serverID uint,
) (*ServerNode, error) {
	server, err := r.finder.FindUserServer(ctx, user, serverID)
	if err != nil {
		return nil, err
	}

	if abilityErr := r.checker.CheckOrError(ctx, user.ID, server.ID, []domain.AbilityName{
		domain.AbilityNameGameServerFiles,
	}); abilityErr != nil {
		return nil, abilityErr
	}

	nodes, findErr := r.nodeRepo.Find(ctx, &filters.FindNode{
		IDs: []uint{server.DSID},
	}, nil, &filters.Pagination{Limit: 1})
	if findErr != nil {
		return nil, errors.WithMessage(findErr, "failed to find node")
	}
	if len(nodes) == 0 {
		return nil, api.NewNotFoundError("node not found")
	}

	return &ServerNode{Server: server, Node: &nodes[0]}, nil
}

var ErrUserNotAuthenticated = api.NewError(http.StatusUnauthorized, "user not authenticated")
