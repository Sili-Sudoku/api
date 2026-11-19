package model

import "time"

// UserJSON is how the API shows a user to the signed-in user.
type UserJSON struct {
	Username  string    `json:"username"`
	Email     string    `json:"email,omitempty"`
	Bio       string    `json:"bio"`
	CreatedAt time.Time `json:"createdAt"`
	IsAdmin   bool      `json:"isAdmin"`
}

func NewUserJSON(u *User) UserJSON {
	return UserJSON{Username: u.Username, Email: u.Email, Bio: u.Bio, CreatedAt: u.CreatedAt, IsAdmin: u.IsAdmin}
}
