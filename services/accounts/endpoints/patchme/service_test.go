package patchme

import (
	"context"
	"errors"
	"strings"
	"testing"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/services/accounts/model"
)

func ptr[T any](v T) *T { return &v }

func okAuth(t *testing.T) *mp.UserAuth {
	return &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*model.User, error) {
		return &model.User{ID: 7, Username: "ann", Bio: "old"}, nil
	}}
}

func TestNew(t *testing.T) {
	auth, db := &mp.UserAuth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "bio too long", Input: &EndpointReq{Bio: ptr(strings.Repeat("a", model.MaxBio+1))}, WantErr: true},
		{Name: "bio is not utf8", Input: &EndpointReq{Bio: ptr("a\xffb")}, WantErr: true},
		{Name: "bio has a NUL", Input: &EndpointReq{Bio: ptr("a\x00b")}, WantErr: true},
		{Name: "bio at the limit", Input: &EndpointReq{Bio: ptr(strings.Repeat("é", model.MaxBio))}},
		{Name: "padding does not count", Input: &EndpointReq{Bio: ptr("  " + strings.Repeat("a", model.MaxBio) + "  ")}},
		{Name: "empty bio", Input: &EndpointReq{Bio: ptr("")}},
		{Name: "no bio", Input: &EndpointReq{}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{
				auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*model.User, error) { return nil, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{Bio: ptr("hi")},
			WantErr: true,
		},
		{
			Name: "db.User.SetBio_error",
			Endpoint: &Endpoint{
				auth: okAuth(t),
				db: &DB{User: &mp.User{T: t, SetBioFunc: func(ctx context.Context, id int64, bio string) error {
					return errors.New("dbErr")
				}}},
			},
			Input:   &EndpointReq{Bio: ptr("hi")},
			WantErr: true,
		},
		{
			Name: "success_trims_and_stores_the_bio",
			Endpoint: &Endpoint{
				auth: okAuth(t),
				db: &DB{User: &mp.User{T: t, SetBioFunc: func(ctx context.Context, id int64, bio string) error {
					if id != 7 || bio != "hi" {
						t.Errorf("SetBio(%d, %q)", id, bio)
					}
					return nil
				}}},
			},
			Input:    &EndpointReq{Bio: ptr("  hi \n")},
			WantResp: &EndpointResp{User: model.UserJSON{Username: "ann", Bio: "hi"}},
		},
		{
			Name:     "success_without_a_bio_changes_nothing",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: &mp.User{T: t}}},
			Input:    &EndpointReq{},
			WantResp: &EndpointResp{User: model.UserJSON{Username: "ann", Bio: "old"}},
		},
	})
}
