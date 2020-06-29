package factory

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/go-scm/scm/driver/fake"

	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx-remote/pkg/fakes/fakeauth"
)

func TestKindResolver_verifyPipelineUser(t *testing.T) {
	r := getResolver(t)

	fileName := filepath.Join("test_data", "succesfully_add_user", "secrets.yaml")
	data, err := ioutil.ReadFile(fileName)
	assert.NoError(t, err, "failed to read file "+fileName)

	err = r.verifyPipelineUser(string(data))
	assert.NoError(t, err, "failed to verify pipeline user")
}

func TestKindResolver_ensurePipelineUserIsCollaboratorIsOrgAdmin(t *testing.T) {
	r := getResolver(t)
	client, _ := fake.NewDefault()
	err := r.ensurePipelineUserIsCollaborator(client, "foo", "foo/bar", "adminUser")
	assert.NoError(t, err, "adminUser should not fail test as it is an admin therefore does not need to be added to repo")

	ctx := context.Background()
	added, _, err := client.Repositories.IsCollaborator(ctx, "foo/bar", "adminUser")
	assert.NoError(t, err, "should not fail")
	assert.False(t, added, "user should not have been added as it is not an owner")
}

func TestKindResolver_ensurePipelineUserIsCollaboratorIsNotOrgAdmin(t *testing.T) {
	r := getResolver(t)
	client, _ := fake.NewDefault()
	err := r.ensurePipelineUserIsCollaborator(client, "foo", "foo/bar", "not_org_admin")
	assert.NoError(t, err, "adminUser should not fail test as it is an admin therefore does not need to be added to repo")

	ctx := context.Background()
	added, _, err := client.Repositories.IsCollaborator(ctx, "foo/bar", "not_org_admin")
	assert.NoError(t, err, "should not fail")
	assert.True(t, added, "user should have been added as it is not an owner")

}

func TestKindResolver_ensurePipelineUserIsCollaboratorIsAdded(t *testing.T) {
	r := getResolver(t)
	client, _ := fake.NewDefault()
	ctx := context.Background()

	added, _, err := client.Repositories.IsCollaborator(ctx, "foo/bar", "cheese_bot")
	assert.NoError(t, err, "should not fail")
	assert.False(t, added, "user should not exist at the start of the test")

	err = r.ensurePipelineUserIsCollaborator(client, "foo", "foo/bar", "cheese_bot")
	assert.NoError(t, err, "should not fail")

	added, _, err = client.Repositories.IsCollaborator(ctx, "foo/bar", "cheese_bot")
	assert.NoError(t, err, "should not fail")
	assert.True(t, added, "user should have been added")

}

func TestKindResolver_ensurePipelineUserIsCollaboratorExistingUserHasReadPermission(t *testing.T) {
	r := getResolver(t)
	client, _ := fake.NewDefault()
	ctx := context.Background()

	// setup
	added, alreadyExisted, _, err := client.Repositories.AddCollaborator(ctx, "foo/bar", "cheese_bot", "read")
	assert.NoError(t, err, "should not fail")
	assert.False(t, alreadyExisted, "user should not have already existed")
	assert.True(t, added, "user should be added")

	permission, _, err := client.Repositories.FindUserPermission(ctx, "foo/bar", "cheese_bot")
	assert.Equal(t, "read", permission, "initial permission should be read")

	// run test
	err = r.ensurePipelineUserIsCollaborator(client, "foo", "foo/bar", "cheese_bot")
	assert.NoError(t, err, "should not get error, we are updating existing read permission to admin")

	// checks
	isCollaborator, _, err := client.Repositories.IsCollaborator(ctx, "foo/bar", "cheese_bot")
	assert.NoError(t, err, "should not fail")
	assert.True(t, isCollaborator, "user is a collaborator")

	permission, _, err = client.Repositories.FindUserPermission(ctx, "foo/bar", "cheese_bot")
	assert.Equal(t, "admin", permission, "permission should have changed from read to admin")

}

func getResolver(t *testing.T) *KindResolver {
	authConfigSvc := fakeauth.NewFakeAuthConfigService(t, "rawlingsj", "dummytoken", "https://fake.git")
	r := &KindResolver{
		GitURL:            "https://fake.git/rawlingsj/foo",
		AuthConfigService: authConfigSvc,
		BatchMode:         true,
		Kind:              "fake",
	}
	return r
}
