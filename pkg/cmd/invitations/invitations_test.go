package invitations

import (
	"reflect"
	"testing"
)

func TestOptions_getInviteDetailsToAccept(t *testing.T) {

	type args struct {
		invites []string
		kind    inviteType
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{name: "simple_org", args: struct {
			invites []string
			kind    inviteType
		}{
			invites: []string{"1. Organization invite: https://github.com/jenkins-x"}, kind: organisation}, want: []string{"jenkins-x"}, wantErr: false,
		},
		{name: "simple_repo", args: struct {
			invites []string
			kind    inviteType
		}{
			invites: []string{"1. Organization invite: https://github.com/jenkins-x/jx"}, kind: repository}, want: []string{"jenkins-x/jx"}, wantErr: false,
		},
		{name: "failure_org", args: struct {
			invites []string
			kind    inviteType
		}{
			invites: []string{"1. Organization invite: https://github.com"}, kind: organisation}, want: nil, wantErr: true,
		},
		{name: "failure_repo", args: struct {
			invites []string
			kind    inviteType
		}{
			invites: []string{"1. Organization invite: https://github.com"}, kind: repository}, want: nil, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Options{}
			got, err := o.getInviteDetailsToAccept(tt.args.invites, tt.args.kind)
			if (err != nil) != tt.wantErr {
				t.Errorf("getInviteDetailsToAccept() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getInviteDetailsToAccept() got = %v, want %v", got, tt.want)
			}
		})
	}
}
