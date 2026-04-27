package inmemory

import (
	"context"
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/repositories"
	repotesting "github.com/gameap/gameap/internal/repositories/testing"
	"github.com/stretchr/testify/suite"
)

func TestRBACRepository(t *testing.T) {
	suite.Run(t, repotesting.NewRBACRepositorySuite(
		func(_ *testing.T) (repositories.RBACRepository, func(ctx context.Context, t *testing.T, name string) domain.Role, func(ctx context.Context, t *testing.T, ability domain.Ability) uint) {
			repo := NewRBACRepository()

			createRoleFunc := func(_ context.Context, t *testing.T, name string) domain.Role {
				t.Helper()

				role := domain.Role{
					ID:    uint(repo.nextRoleID.Add(1)),
					Name:  name,
					Title: new(name + " Title"),
					Level: new(uint(1)),
					Scope: new(1),
				}

				repo.mu.Lock()
				repo.roles[role.ID] = &role
				repo.mu.Unlock()

				return role
			}

			createAbilityFunc := func(_ context.Context, t *testing.T, ability domain.Ability) uint {
				t.Helper()

				ability.ID = uint(repo.nextAbilityID.Add(1))

				repo.mu.Lock()
				repo.abilities[ability.ID] = &ability
				repo.mu.Unlock()

				return ability.ID
			}

			return repo, createRoleFunc, createAbilityFunc
		},
	))
}
