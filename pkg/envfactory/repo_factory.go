package envfactory

import (
	"context"
	"fmt"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

type CreateRepository struct {
	GitServer       string
	GitKind         string
	Owner           string
	Repository      string
	CurrentUsername string
	GitPublic       bool
}

// ConfirmValues confirms to the user the values to be used to create the new git repository
// if using batch mode lets just validate the values are supplied or fail
func (r *CreateRepository) ConfirmValues(batch bool, handles util.IOFileHandles) error {
	if batch {
		if r.GitServer == "" {
			return util.MissingOption("git-server")
		}
		if r.Owner == "" {
			return util.MissingOption("env-git-owner")
		}
		if r.Repository == "" {
			return util.MissingOption("repo")
		}
		return nil
	}
	var err error
	r.GitServer, err = util.PickValue("git server for the new git repository:", r.GitServer, true, "", handles)
	if err != nil {
		return err
	}

	saasGitKind := gits.SaasGitKind(r.GitServer)
	if saasGitKind != "" {
		r.GitKind = saasGitKind
	} else {
		message := fmt.Sprintf("kind of the git server (%s):", r.GitServer)
		r.GitKind, err = util.PickNameWithDefault(gits.KindGits, message, r.GitKind, "we need to know what kind of git provider this server is so we know what kind of REST API to use", handles)
		if err != nil {
			return err
		}
	}

	r.Owner, err = util.PickValue("git owner (user/organization) for the new git repository:", r.Owner, true, "", handles)
	if err != nil {
		return err
	}

	r.Repository, err = util.PickValue("git repository name:", r.Repository, true, "", handles)
	if err != nil {
		return err
	}
	return nil
}

// CreateRepository creates the git repository if it does not already exist
func (r *CreateRepository) CreateRepository(scmClient *scm.Client) (*scm.Repository, error) {
	info := util.ColorInfo
	log.Logger().Infof("checking git repository %s/%s exists on server %s", info(r.Owner), info(r.Repository), info(r.GitServer))

	ctx := context.Background()
	fullName := r.FullName()

	repo, _, err := scmClient.Repositories.Find(ctx, fullName)
	if IsScmNotFound(err) {
		err = nil
		repo = nil
	}
	if err != nil {
		return repo, errors.Wrapf(err, "failed to lookup repository %s", fullName)
	}
	if repo != nil {
		log.Logger().Infof("repository already exists at %s", info(repo.Link))
		return repo, nil
	}

	log.Logger().Infof("creating git repository %s/%s on server %s", info(r.Owner), info(r.Repository), info(r.GitServer))

	input := &scm.RepositoryInput{
		Name:    r.Repository,
		Private: !r.GitPublic,
	}
	// only specify owner if its not the current user
	if r.CurrentUsername != r.Owner {
		input.Namespace = r.Owner
	}
	repo, _, err = scmClient.Repositories.Create(ctx, input)
	if err != nil {
		return repo, errors.Wrapf(err, "failed to create repository %s", fullName)
	}

	log.Logger().Infof("creating git repository %s", info(repo.Link))
	return repo, nil
}

func (r *CreateRepository) FullName() string {
	return scm.Join(r.Owner, r.Repository)
}

func IsScmNotFound(err error) bool {
	if err != nil {
		// I think that we should instead rely on the http status (404)
		// until jenkins-x go-scm is updated t return that in the error this works for github and gitlab
		return strings.Contains(err.Error(), scm.ErrNotFound.Error())
	}
	return false
}
