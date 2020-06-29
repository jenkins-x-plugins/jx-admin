package fakeauth

import (
	"testing"

	"github.com/jenkins-x/jx/v2/pkg/auth"
	"github.com/stretchr/testify/require"
)

// NewFakeAuthConfigService creates a fake git auth service
func NewFakeAuthConfigService(t *testing.T, username string, token string, serverURLs ...string) auth.ConfigService {
	svc := auth.NewMemoryAuthConfigService()
	require.NotNil(t, svc, "no auth.ConfigService")
	cfg := svc.Config()
	require.NotNil(t, svc, "auth.ConfigService has no Config")

	for _, serverURL := range serverURLs {
		server := cfg.GetOrCreateServer(serverURL)
		server.Users = append(server.Users, &auth.UserAuth{
			Username: username,
			ApiToken: token,
		})
		server.CurrentUser = username
	}
	cfg.DefaultUsername = username
	return svc
}
