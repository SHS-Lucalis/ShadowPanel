package hostlibrary

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/plugin/sdk/users"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type UsersServiceImpl struct {
	userRepo repositories.UserRepository
}

func NewUsersService(userRepo repositories.UserRepository) *UsersServiceImpl {
	return &UsersServiceImpl{
		userRepo: userRepo,
	}
}

func (s *UsersServiceImpl) FindUsers(
	ctx context.Context,
	req *users.FindUsersRequest,
) (*users.FindUsersResponse, error) {
	var filter *filters.FindUser
	if req.Filter != nil {
		filter = &filters.FindUser{
			IDs:    uintsFromUint64s(req.Filter.Ids),
			Logins: []string{},
			Emails: []string{},
		}
		if req.Filter.Login != nil {
			filter.Logins = []string{*req.Filter.Login}
		}
		if req.Filter.Email != nil {
			filter.Emails = []string{*req.Filter.Email}
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

	result, err := s.userRepo.Find(ctx, filter, sorting, pagination)
	if err != nil {
		return nil, err
	}

	return &users.FindUsersResponse{
		Users: convertUsersToProto(result),
		Total: int32(len(result)), //nolint:gosec
	}, nil
}

func (s *UsersServiceImpl) GetUser(
	ctx context.Context,
	req *users.GetUserRequest,
) (*users.GetUserResponse, error) {
	result, err := s.userRepo.Find(ctx, filters.FindUserByIDs(uint(req.Id)), nil, nil)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return &users.GetUserResponse{Found: false}, nil
	}

	return &users.GetUserResponse{
		User:  convertUserToProto(&result[0]),
		Found: true,
	}, nil
}

func convertUsersToProto(usrs []domain.User) []*proto.User {
	return lo.Map(usrs, func(u domain.User, _ int) *proto.User {
		return convertUserToProto(&u)
	})
}

func convertUserToProto(u *domain.User) *proto.User {
	return &proto.User{
		Id:    uint64(u.ID),
		Login: u.Login,
		Email: u.Email,
		Name:  u.Name,
	}
}

type UsersHostLibrary struct {
	impl *UsersServiceImpl
}

func NewUsersHostLibrary(userRepo repositories.UserRepository) *UsersHostLibrary {
	return &UsersHostLibrary{
		impl: NewUsersService(userRepo),
	}
}

func (l *UsersHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return users.Instantiate(ctx, r, l.impl)
}
