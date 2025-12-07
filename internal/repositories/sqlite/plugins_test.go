package sqlite_test

import (
	"testing"

	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/repositories/sqlite"
	repotesting "github.com/gameap/gameap/internal/repositories/testing"
	"github.com/stretchr/testify/suite"
)

func TestPluginRepository(t *testing.T) {
	suite.Run(t, repotesting.NewPluginRepositorySuite(
		func(t *testing.T) repositories.PluginRepository {
			t.Helper()

			return sqlite.NewPluginRepository(SetupTestDB(t))
		},
	))
}
