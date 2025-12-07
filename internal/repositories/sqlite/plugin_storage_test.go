package sqlite_test

import (
	"testing"

	"github.com/gameap/gameap/internal/repositories"
	repotesting "github.com/gameap/gameap/internal/repositories/testing"
	"github.com/stretchr/testify/suite"

	"github.com/gameap/gameap/internal/repositories/sqlite"
)

func TestPluginStorageRepository(t *testing.T) {
	suite.Run(t, repotesting.NewPluginStorageRepositorySuite(
		func(t *testing.T) repositories.PluginStorageRepository {
			t.Helper()

			return sqlite.NewPluginStorageRepository(SetupTestDB(t))
		},
	))
}
