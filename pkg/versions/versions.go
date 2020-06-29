package versions

import (
	"io/ioutil"

	"github.com/jenkins-x/jx-remote/pkg/common"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/jenkins-x/jx/pkg/versionstream/versionstreamrepo"
	"github.com/pkg/errors"
)

// GetDefaultVersionNumber returns the version number for the given kind and name using the default version stream
func GetDefaultVersionNumber(kind versionstream.VersionKind, name string) (string, error) {
	return GetVersionNumber(kind, name, config.DefaultVersionsURL, "master", nil, common.GetIOFileHandles(nil))
}

// GetVersionNumber returns the version number for the given kind and name or blank string if there is no locked version
func GetVersionNumber(kind versionstream.VersionKind, name, repo, gitRef string, git gits.Gitter, handles util.IOFileHandles) (string, error) {
	if git == nil {
		git = gits.NewGitCLI()
	}
	versioner, err := CreateVersionResolver(repo, gitRef, git, handles)
	if err != nil {
		return "", err
	}
	return versioner.StableVersionNumber(kind, name)
}

// CreateVersionResolver creates a new VersionResolver service
func CreateVersionResolver(versionRepository string, versionRef string, git gits.Gitter, handles util.IOFileHandles) (*versionstream.VersionResolver, error) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary dir for version stream")
	}

	versionsDir, _, err := versionstreamrepo.CloneJXVersionsRepoToDir(tempDir, versionRepository, versionRef, nil, git, true, false, handles)
	if err != nil {
		return nil, err
	}
	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}, nil
}
