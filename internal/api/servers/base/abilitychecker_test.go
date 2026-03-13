package base_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	serversbase "github.com/gameap/gameap/internal/api/servers/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/rbac"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRBAC(t *testing.T) (*rbac.RBAC, *inmemory.RBACRepository) {
	t.Helper()

	repo := inmemory.NewRBACRepository()
	tm := services.NewNilTransactionManager()
	rbacService := rbac.NewRBAC(tm, repo, 1*time.Minute)

	return rbacService, repo
}

func createAdminRole(t *testing.T, repo *inmemory.RBACRepository) domain.Role {
	t.Helper()

	role := domain.Role{
		Name:  "admin",
		Title: new("Administrator"),
	}

	err := repo.SaveRole(context.Background(), &role)
	require.NoError(t, err)

	ability := domain.Ability{
		Name: domain.AbilityNameAdminRolesPermissions,
	}
	err = repo.SaveAbility(context.Background(), &ability)
	require.NoError(t, err)

	err = repo.Allow(
		context.Background(),
		role.ID,
		domain.EntityTypeRole,
		[]domain.Ability{ability},
	)
	require.NoError(t, err)

	return role
}

func assignRoleToUser(t *testing.T, repo *inmemory.RBACRepository, userID uint, role domain.Role) {
	t.Helper()

	err := repo.AssignRolesForEntity(
		context.Background(),
		userID,
		domain.EntityTypeUser,
		[]domain.RestrictedRole{domain.NewRestrictedRoleFromRole(role)},
	)
	require.NoError(t, err)
}

func allowUserAbilityForServer(
	t *testing.T,
	repo *inmemory.RBACRepository,
	userID uint,
	serverID uint,
	abilityName domain.AbilityName,
) {
	t.Helper()

	ability := domain.CreateAbilityForEntity(abilityName, serverID, domain.EntityTypeServer)
	err := repo.SaveAbility(context.Background(), &ability)
	require.NoError(t, err)

	err = repo.Allow(
		context.Background(),
		userID,
		domain.EntityTypeUser,
		[]domain.Ability{ability},
	)
	require.NoError(t, err)
}

func TestAbilityChecker_Check(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		serverID      uint
		abilities     []domain.AbilityName
		setup         func(t *testing.T, rbacService *rbac.RBAC, repo *inmemory.RBACRepository)
		expected      bool
		expectedError string
	}{
		{
			name:      "admin_user_has_access",
			userID:    1,
			serverID:  10,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStart},
			setup: func(t *testing.T, _ *rbac.RBAC, repo *inmemory.RBACRepository) {
				t.Helper()
				adminRole := createAdminRole(t, repo)
				assignRoleToUser(t, repo, 1, adminRole)
			},
			expected:      true,
			expectedError: "",
		},
		{
			name:      "non_admin_user_with_single_ability",
			userID:    2,
			serverID:  20,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStop},
			setup: func(t *testing.T, _ *rbac.RBAC, repo *inmemory.RBACRepository) {
				t.Helper()
				allowUserAbilityForServer(t, repo, 2, 20, domain.AbilityNameGameServerStop)
			},
			expected:      true,
			expectedError: "",
		},
		{
			name:      "non_admin_user_with_multiple_abilities",
			userID:    3,
			serverID:  30,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStart, domain.AbilityNameGameServerStop},
			setup: func(t *testing.T, _ *rbac.RBAC, repo *inmemory.RBACRepository) {
				t.Helper()
				allowUserAbilityForServer(t, repo, 3, 30, domain.AbilityNameGameServerStart)
				allowUserAbilityForServer(t, repo, 3, 30, domain.AbilityNameGameServerStop)
			},
			expected:      true,
			expectedError: "",
		},
		{
			name:      "non_admin_user_without_ability",
			userID:    4,
			serverID:  40,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerFiles},
			setup: func(t *testing.T, _ *rbac.RBAC, _ *inmemory.RBACRepository) {
				t.Helper()
			},
			expected:      false,
			expectedError: "",
		},
		{
			name:      "non_admin_user_with_partial_abilities",
			userID:    5,
			serverID:  50,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStart, domain.AbilityNameGameServerStop},
			setup: func(t *testing.T, _ *rbac.RBAC, repo *inmemory.RBACRepository) {
				t.Helper()
				allowUserAbilityForServer(t, repo, 5, 50, domain.AbilityNameGameServerStart)
			},
			expected:      false,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rbacService, repo := setupRBAC(t)
			defer rbacService.Close()

			if tt.setup != nil {
				tt.setup(t, rbacService, repo)
			}

			checker := serversbase.NewAbilityChecker(rbacService)
			result, err := checker.Check(context.Background(), tt.userID, tt.serverID, tt.abilities)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAbilityChecker_CheckOrError(t *testing.T) {
	tests := []struct {
		name               string
		userID             uint
		serverID           uint
		abilities          []domain.AbilityName
		setup              func(t *testing.T, rbacService *rbac.RBAC, repo *inmemory.RBACRepository)
		expectedError      string
		expectedStatusCode int
	}{
		{
			name:      "user_has_single_ability",
			userID:    1,
			serverID:  10,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStart},
			setup: func(t *testing.T, _ *rbac.RBAC, repo *inmemory.RBACRepository) {
				t.Helper()
				adminRole := createAdminRole(t, repo)
				assignRoleToUser(t, repo, 1, adminRole)
			},
			expectedError:      "",
			expectedStatusCode: 0,
		},
		{
			name:      "user_has_multiple_abilities",
			userID:    2,
			serverID:  20,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStart, domain.AbilityNameGameServerStop},
			setup: func(t *testing.T, _ *rbac.RBAC, repo *inmemory.RBACRepository) {
				t.Helper()
				allowUserAbilityForServer(t, repo, 2, 20, domain.AbilityNameGameServerStart)
				allowUserAbilityForServer(t, repo, 2, 20, domain.AbilityNameGameServerStop)
			},
			expectedError:      "",
			expectedStatusCode: 0,
		},
		{
			name:      "user_does_not_have_ability",
			userID:    3,
			serverID:  30,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStop},
			setup: func(t *testing.T, _ *rbac.RBAC, _ *inmemory.RBACRepository) {
				t.Helper()
			},
			expectedError:      "user does not have required permissions",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:      "user_has_partial_abilities",
			userID:    4,
			serverID:  40,
			abilities: []domain.AbilityName{domain.AbilityNameGameServerStart, domain.AbilityNameGameServerStop},
			setup: func(t *testing.T, _ *rbac.RBAC, repo *inmemory.RBACRepository) {
				t.Helper()
				allowUserAbilityForServer(t, repo, 4, 40, domain.AbilityNameGameServerStart)
			},
			expectedError:      "user does not have required permissions",
			expectedStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rbacService, repo := setupRBAC(t)
			defer rbacService.Close()

			if tt.setup != nil {
				tt.setup(t, rbacService, repo)
			}

			checker := serversbase.NewAbilityChecker(rbacService)
			err := checker.CheckOrError(context.Background(), tt.userID, tt.serverID, tt.abilities)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)

				if tt.expectedStatusCode != 0 {
					type httpStatusError interface {
						HTTPStatus() int
					}
					httpErr, ok := err.(httpStatusError)
					require.True(t, ok, "error should have HTTPStatus method")
					assert.Equal(t, tt.expectedStatusCode, httpErr.HTTPStatus())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
