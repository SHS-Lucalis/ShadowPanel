package importgameap

import (
	"net/http"

	"github.com/gameap/gameap/internal/domain/gamesimport"
	"github.com/gameap/gameap/pkg/api"
	"github.com/samber/lo"
)

type input struct {
	Name *string
	Code *string
}

func readInput(r *http.Request) (*input, error) {
	reader := api.NewQueryReader(r)

	name, err := reader.ReadString("name")
	if err != nil {
		return nil, err
	}

	code, err := reader.ReadString("code")
	if err != nil {
		return nil, err
	}

	return &input{
		Name: lo.Ternary(name != "", &name, nil),
		Code: lo.Ternary(code != "", &code, nil),
	}, nil
}

func (i *input) toImportOptions() *gamesimport.Options {
	if i == nil || (i.Name == nil && i.Code == nil) {
		return nil
	}

	return &gamesimport.Options{
		Name: i.Name,
		Code: i.Code,
	}
}
