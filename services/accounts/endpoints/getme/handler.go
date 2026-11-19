package getme

import (
	"context"

	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type Endpoint struct {
	auth Auth
}

func New(auth Auth) *Endpoint { return &Endpoint{auth: auth} }

type Auth interface {
	GetUser(ctx context.Context) (*model.User, error)
}
