package upgrade_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenkins-x/jx-admin/pkg/cmd/upgrade"
	"github.com/jenkins-x/jx-admin/pkg/fakes/fakeauth"
	"github.com/jenkins-x/jx-admin/pkg/testhelpers"
	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	v1fake "github.com/jenkins-x/jx-api/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner/fakerunner"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

func TestUpgrade(t *testing.T) {
	//t.Parallel()

	ns := "jx"
	sourceDir := filepath.Join("test_data")

	testDirs, err := ioutil.ReadDir(sourceDir)
	require.NoError(t, err, "failed to read dir %s", sourceDir)
	for _, d := range testDirs {
		name := d.Name()
		if !d.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		t.Logf("running test %s\n", name)

		testDir := filepath.Join(sourceDir, name)
		envDir := filepath.Join(testDir, "env")

		files, err := ioutil.ReadDir(envDir)
		require.NoError(t, err, "failed to read dir %s", envDir)

		var kubeObjects []runtime.Object
		var jxObjects []runtime.Object
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".yaml" {
				e := &v1.Environment{}
				fileName := filepath.Join(envDir, f.Name())
				t.Logf("loading environment %s", fileName)
				data, err := ioutil.ReadFile(fileName)
				require.NoError(t, err, "failed to load environment %s", fileName)

				err = yaml.Unmarshal(data, e)
				require.NoError(t, err, "failed to unmarshal environment %s", fileName)
				e.Namespace = ns
				jxObjects = append(jxObjects, e)
			}
		}

		_, uo := upgrade.NewCmdUpgrade()
		uo.BatchMode = true
		runner := &fakerunner.FakeRunner{
			CommandRunner: func(c *cmdrunner.Command) (string, error) {
				args := c.Args
				if len(args) > 0 {
					switch args[0] {
					case "push":
						t.Logf("ignoring command: %s\n", c.CLI())
						return "", nil
					default:
						return cmdrunner.DefaultCommandRunner(c)
					}
				}
				t.Logf("ignoring command: %s\n", c.CLI())
				return "", nil
			},
		}
		uo.Gitter = cli.NewCLIClient("", runner.Run)
		uo.KubeClient = fake.NewSimpleClientset(kubeObjects...)
		uo.JXClient = v1fake.NewSimpleClientset(jxObjects...)
		uo.Namespace = ns

		createRepo := name == "jx-install"
		fullName := "jstrachan/environment-mycluster-dev"
		gitServerURL := "https://github.com"
		if createRepo {
			fullName = "myorg/dummy"
			uo.RepoName = "dummy"
			uo.OverrideRequirements.Cluster.GitKind = "fake"
			gitServerURL = "https://fake.com"
			uo.OverrideRequirements.Cluster.GitServer = gitServerURL
		} else {
			uo.UsePullRequest = true
		}
		uo.EnvFactory.AuthConfigService = fakeauth.NewFakeAuthConfigService(t, "jstrachan", "dummytoken", gitServerURL)

		err = uo.Run()
		require.NoError(t, err, "failed to upgrade repository")

		scmClient := uo.EnvFactory.ScmClient
		require.NotNil(t, scmClient, "no ScmClient created")

		ctx := context.Background()
		if createRepo {
			// now lets assert we created a new repository
			repo, _, err := scmClient.Repositories.Find(ctx, fullName)
			require.NoError(t, err, "failed to find repository %s", fullName)
			assert.NotNil(t, repo, "nil repository %s", fullName)
			assert.Equal(t, fullName, repo.FullName, "repo.FullName")
			assert.Equal(t, uo.RepoName, repo.Name, "repo.FullName")
		} else {
			// lets assert we created a Pull Request
			pr, _, err := scmClient.PullRequests.Find(ctx, fullName, 1)
			require.NoError(t, err, "failed to find repository %s", fullName)
			assert.NotNil(t, pr, "nil pr %s", fullName)

			t.Logf("created PullRequest %s", pr.Link)
		}

		dir := uo.OutDir
		assert.NotEmpty(t, dir, "no output dir generated")
		actualReq, actualReqFile, err := config.LoadRequirementsConfig(dir, false)
		assert.NoError(t, err, "failed to load generated requirements in dir %s for %s", dir, name)
		assert.NotEmpty(t, actualReqFile, "no requirements file found for test %s in output dir %s", dir)

		expectedFile := filepath.Join(testDir, "expected-jx-requirements.yml")
		_, err = config.LoadRequirementsConfigFile(expectedFile, false)
		assert.NoError(t, err, "failed to load expected requirements file %s for %s", expectedFile, name)
		assert.FileExists(t, expectedFile, "expected requirements file for test %s", name)

		//testhelpers.AssertYamlEqual(t, expectedFile, actualReqFile, "requirements for test %s", name)

		// lets change the version stream tag to the dummy value so we can compare them better
		switch name {
		case "jx-install":
			actualReq.Cluster.GitName = ""
			actualReq.Cluster.Namespace = ""
			actualReq.Ingress.NamespaceSubDomain = ""
			actualReq.Repository = config.RepositoryTypeUnknown
			actualReq.VersionStream.Ref = ""
		case "jx-boot-gitops":
			actualReq.VersionStream.Ref = "master"
		default:
			actualReq.VersionStream.Ref = "mysha1234"
		}
		err = actualReq.SaveConfig(actualReqFile)
		require.NoError(t, err, "failed to save %s", actualReqFile)

		testhelpers.AssertTextFilesEqual(t, expectedFile, actualReqFile, fmt.Sprintf("requirements for test %s", name))

		/* TODO
		if name == "helmfile" {
			projectConfig, _, err := config.LoadProjectConfig(dir)
			require.NoError(t, err, "failed to load project config from %s for %s", dir, name)
			assert.Equal(t, "0.0.14", projectConfig.BuildPackGitURef, "projectConfig.BuildPackGitURef for %s", name)
		}
		*/
	}
}
