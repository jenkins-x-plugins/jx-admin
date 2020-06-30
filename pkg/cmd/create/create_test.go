package create_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	v1fake "github.com/jenkins-x/jx-api/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-apps/pkg/jxapps"
	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner/fakerunner"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/cli"
	"github.com/jenkins-x/jx-remote/pkg/cmd/create"
	"github.com/jenkins-x/jx-remote/pkg/fakes/fakeauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	//t.Parallel()

	type testCase struct {
		Name        string
		Environment string
		Args        []string
	}
	testCases := []testCase{
		{
			Name:        "staging-pr",
			Environment: "staging-pr",
			Args:        []string{"--provider", "kubernetes", "--env-git-public", "--git-public", "--dev-git-kind=fake", "--dev-git-url", "https://github.com/jstrachan/environment-fake-dev.git"},
		},
		{
			Name:        "mystaging",
			Environment: "staging",
			Args:        []string{"--provider", "kubernetes", "--env-git-public", "--git-public"},
		},
		{
			Name:        "myproduction",
			Environment: "production",
			Args:        []string{"--provider", "kubernetes", "--env-git-public", "--git-public"},
		},
		{
			Name: "add-remove",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--add=jx-labs/istio", "--add=flagger/flagger", "--remove=stable/nginx-ingress"},
		},
		{
			Name: "vault",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--secret", "vault"},
		},
		{
			Name: "remote",
			Args: []string{"--provider", "kubernetes", "--env-git-public", "--git-public", "--env-remote"},
		},
		{
			Name: "bucketrepo",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--repository", "bucketrepo"},
		},
		{
			Name: "tls",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--tls", "--tls-production"},
		},
		{
			Name: "tls-custom-secret",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--tls", "--tls-secret", "my-tls-secret"},
		},
		/* TODO
		{
			Name: "canary",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--canary", "--hpa"},
		},
		{
			Name: "istio",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--ingress-kind=istio"},
		},
		*/
		{
			Name: "kubernetes",
			Args: []string{"--provider", "kubernetes", "--env-git-public", "--git-public"},
		},
	}

	for _, tc := range testCases {
		t.Logf("running test: %s", tc.Name)
		_, co := create.NewCmdCreate()
		co.BatchMode = true
		co.NoOperator = true

		runner := &fakerunner.FakeRunner{
			CommandRunner: func(c *cmdrunner.Command) (string, error) {
				args := c.Args
				if len(args) > 0 && args[0] == "clone" {
					// lets really git clone but then fake out all other commands
					return cmdrunner.DefaultCommandRunner(c)
				}
				return "", nil
			},
		}
		co.Gitter = cli.NewCLIClient("", runner.Run)
		co.DisableVerifyPackages = true
		outFile, err := ioutil.TempFile("", "")
		require.NoError(t, err, "failed to create tempo file")
		outFileName := outFile.Name()
		args := append(tc.Args, "--git-server", "https://fake.com", "--git-kind", "fake", "--env-git-owner", "jstrachan", "--cluster", tc.Name, "--out", outFileName)
		args = append(args, tc.Args...)
		co.Args = args
		co.Environment = tc.Environment
		if !create.SupportHelm3ForDev && co.Environment == "" {
			co.Environment = "dev"
		}
		repoName := fmt.Sprintf("environment-%s-%s", tc.Name, co.Environment)
		if co.RepoName == "" {
			co.RepoName = repoName
		}
		co.JXClient = v1fake.NewSimpleClientset()
		co.EnvFactory.AuthConfigService = fakeauth.NewFakeAuthConfigService(t, "jstrachan", "dummytoken", "https://fake.com", "https://github.com")

		err = co.Run()
		require.NoError(t, err, "failed to create repository for test %s", tc.Name)

		// now lets assert we created a new repository
		ctx := context.Background()
		fullName := fmt.Sprintf("jstrachan/%s", repoName)

		repo, _, err := co.EnvFactory.ScmClient.Repositories.Find(ctx, fullName)
		require.NoError(t, err, "failed to find repository %s", fullName)
		assert.NotNil(t, repo, "nil repository %s", fullName)
		assert.Equal(t, fullName, repo.FullName, "repo.FullName for %s", tc.Name)
		assert.Equal(t, repoName, repo.Name, "repo.FullName for %s", tc.Name)

		t.Logf("test %s created dir %s\n", tc.Name, co.OutDir)

		apps, appFileName, err := jxapps.LoadAppConfig(co.OutDir)
		require.NoError(t, err, "failed to load the apps configuration in dir %s for test %s", co.OutDir, tc.Name)
		appMessage := fmt.Sprintf("test %s for file %s", tc.Name, appFileName)

		AssertHasApp(t, apps, "jenkins-x/lighthouse", appMessage)

		switch tc.Name {
		case "mystaging":
			AssertNoApp(t, apps, "jenkins-x/chartmuseum", appMessage)
			AssertNoApp(t, apps, "jenkins-x/nexus", appMessage)
			AssertNoApp(t, apps, "jenkins-x/bucketrepo", appMessage)
			AssertNoApp(t, apps, "repositories", appMessage)
		case "remote":
			AssertHasApp(t, apps, "jenkins-x/chartmuseum", appMessage)
			AssertHasApp(t, apps, "jenkins-x/nexus", appMessage)
			AssertHasApp(t, apps, "repositories", appMessage)
			AssertNoApp(t, apps, "jenkins-x/bucketrepo", appMessage)

		case "bucketrepo":
			AssertHasApp(t, apps, "jenkins-x/bucketrepo", appMessage)
			AssertHasApp(t, apps, "repositories", appMessage)
			AssertNoApp(t, apps, "jenkins-x/chartmuseum", appMessage)
			AssertNoApp(t, apps, "jenkins-x/nexus", appMessage)

		case "tls":
			AssertHasApp(t, apps, "jetstack/cert-manager", appMessage)
			AssertHasApp(t, apps, "jx-labs/acme", appMessage)

		case "tls-custom-secret":
			AssertNoApp(t, apps, "jetstack/cert-manager", appMessage)
			AssertNoApp(t, apps, "jx-labs/acme", appMessage)

		case "istio":
			AssertHasApp(t, apps, "jx-labs/istio", appMessage)
			AssertNoApp(t, apps, "stable/nginx-ingress", appMessage)

		case "add-remove":
			AssertHasApp(t, apps, "jx-labs/istio", appMessage)
			AssertHasApp(t, apps, "flagger/flagger", appMessage)
			AssertNoApp(t, apps, "stable/nginx-ingress", appMessage)

		case "kubernetes":
			AssertHasApp(t, apps, "stable/docker-registry", appMessage)
		}
		assert.FileExists(t, outFileName, "did not generate the Git URL file")
		data, err := ioutil.ReadFile(outFileName)
		require.NoError(t, err, "failed to load file %s", outFileName)
		text := strings.TrimSpace(string(data))
		expectedGitURL := fmt.Sprintf("https://fake.com/jstrachan/environment-%s-%s.git", tc.Name, co.Environment)
		assert.Equal(t, expectedGitURL, text, "output Git URL")

		requirements, _, err := config.LoadRequirementsConfig(co.OutDir, false)
		require.NoError(t, err, "failed to load requirements from %s", co.OutDir)
		assert.Equal(t, true, requirements.Cluster.EnvironmentGitPublic, "requirements.Cluster.EnvironmentGitPublic")
		assert.Equal(t, true, requirements.Cluster.GitPublic, "requirements.Cluster.GitPublic")
		assert.NotEmpty(t, requirements.VersionStream.URL, "requirements.VersionStream.URL for %s", tc.Name)
		assert.NotEmpty(t, string(requirements.SecretStorage), "requirements.SecretStorage for %s", tc.Name)

		switch tc.Name {
		/*
			TODO
			case "canary":
			require.NotNil(t, requirements.DeployOptions, "requirements.DeployOptions is nil for test %s", tc.Name)
			assert.Equal(t, true, requirements.DeployOptions.Canary, "requirements.DeployOptions.Canary for test %s", tc.Name)
			assert.Equal(t, true, requirements.DeployOptions.HPA, "requirements.DeployOptions.HPA for test %s", tc.Name)
			t.Logf("test %s has requirements.DeployOptions %#v", tc.Name, requirements.DeployOptions)
		*/
		case "mystaging":
			require.Equal(t, 1, len(requirements.Environments), "len(requirements.Environments) for tests %s", tc.Name)
			devEnv := requirements.Environments[0]
			assert.Equal(t, true, devEnv.RemoteCluster, "requirements.Environments[0].RemoteCluster for dev with test %s", tc.Name)
			assert.NotEmpty(t, devEnv.Repository, "requirements.Environments[0].Repository for dev with test %s", tc.Name)
			assert.Equal(t, "environment-mystaging-staging", devEnv.Repository, "requirements.Environments[0].Repository for dev with test %s", tc.Name)
			assert.Equal(t, v1.PromotionStrategyTypeAutomatic, devEnv.PromotionStrategy, "requirements.Environments[0].PromotionStrategy for dev with test %s", tc.Name)

		case "staging-pr":
			require.Equal(t, 1, len(requirements.Environments), "len(requirements.Environments) for tests %s", tc.Name)
			devEnv := requirements.Environments[0]
			assert.Equal(t, true, devEnv.RemoteCluster, "requirements.Environments[0].RemoteCluster for dev with test %s", tc.Name)
			assert.NotEmpty(t, devEnv.Repository, "requirements.Environments[0].Repository for dev with test %s", tc.Name)
			assert.Equal(t, "environment-staging-pr-staging-pr", devEnv.Repository, "requirements.Environments[0].Repository for dev with test %s", tc.Name)
			assert.Equal(t, v1.PromotionStrategyTypeManual, devEnv.PromotionStrategy, "requirements.Environments[0].PromotionStrategy for dev with test %s", tc.Name)

		case "myproduction":
			require.Equal(t, 1, len(requirements.Environments), "len(requirements.Environments) for tests %s", tc.Name)
			devEnv := requirements.Environments[0]
			assert.Equal(t, true, devEnv.RemoteCluster, "requirements.Environments[0].RemoteCluster for dev with test %s", tc.Name)
			assert.NotEmpty(t, devEnv.Repository, "requirements.Environments[0].Repository for dev with test %s", tc.Name)
			assert.Equal(t, "environment-myproduction-production", devEnv.Repository, "requirements.Environments[0].Repository for dev with test %s", tc.Name)
			assert.Equal(t, v1.PromotionStrategyTypeManual, devEnv.PromotionStrategy, "requirements.Environments[0].PromotionStrategy for dev with test %s", tc.Name)

		default:
			for i, e := range requirements.Environments {
				if e.Key == "dev" {
					assert.Equal(t, false, e.RemoteCluster, "requirements.Environments[%d].RemoteCluster for key %s", i, e.Key)
				} else {
					expectedRemote := tc.Name == "remote"
					assert.Equal(t, expectedRemote, e.RemoteCluster, "requirements.Environments[%d].RemoteCluster for key %s", i, e.Key)
				}
				t.Logf("requirements.Environments[%d].RemoteCluster = %v for key %s ", i, e.RemoteCluster, e.Key)
			}
		}

		/*
			TODO
			// lets verify we defaulted a serviceType
			if tc.Name == "kubernetes" {
				assert.Equal(t, "LoadBalancer", requirements.Ingress.ServiceType, "requirements.Ingress.ServiceType for test %s", tc.Name)
			}

				if requirements.Cluster.Provider == "kind" {
					assert.Equal(t, true, requirements.Ingress.IgnoreLoadBalancer, "dev requirements.Ingress.IgnoreLoadBalancer for test %s", tc.Name)
				}
		*/

		if tc.Name == "vault" {
			assert.Equal(t, config.SecretStorageTypeVault, requirements.SecretStorage, "dev requirements.SecretStorage for test %s", tc.Name)
			t.Logf("has vault secret storage for test %s", tc.Name)
		}
	}
}

// AssertHasApp asserts that the given app name is in the generated apps YAML
func AssertHasApp(t *testing.T, appConfig *jxapps.AppConfig, appName string, message string) {
	found, names := HasApp(t, appConfig, appName, message)
	if !found {
		assert.Fail(t, fmt.Sprintf("does not have the app %s for %s. Current apps are: %s", appName, message, strings.Join(names, ", ")))
	}
}

// AssertNoApp asserts that the given app name is in the generated apps YAML
func AssertNoApp(t *testing.T, appConfig *jxapps.AppConfig, appName string, message string) {
	found, names := HasApp(t, appConfig, appName, message)
	if found {
		assert.Fail(t, fmt.Sprintf("should not have the app %s for %s. Current apps are: %s", appName, message, strings.Join(names, ", ")))
	}
}

// HasApp tests that the app config has the given app
func HasApp(t *testing.T, appConfig *jxapps.AppConfig, appName string, message string) (bool, []string) {
	found := false
	names := []string{}
	for _, app := range appConfig.Apps {
		names = append(names, app.Name)
		if app.Name == appName {
			t.Logf("has app %s for %s", appName, message)
			found = true
		}
	}
	return found, names
}
