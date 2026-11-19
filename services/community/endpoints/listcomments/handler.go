package listcomments

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

// Auth answers for signed-out callers too: FindUser returns nil then.
type Auth interface {
	FindUser(ctx context.Context) *accounts.User
}

type DB struct {
	Comment CommentDB
	Puzzle  PuzzleDB
	User    UserDB
}

type CommentDB interface {
	List(ctx context.Context, puzzleID string, after *model.CommentCursor, limit int) ([]*model.Comment, error)
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*puzzles.Puzzle, error)
}

type UserDB interface {
	Usernames(ctx context.Context, ids []int64) (map[int64]string, error)
}
