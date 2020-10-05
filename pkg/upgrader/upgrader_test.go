package upgrader_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jenkins-x/jx-admin/pkg/upgrader"
	v1 "github.com/jenkins-x/jx-api/v3/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-api/v3/pkg/config"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestHelmfileUpgradeFromCluster(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join("test_data", "cluster")

	testDirs, err := ioutil.ReadDir(sourceDir)
	require.NoError(t, err, "failed to read dir %s", sourceDir)
	for _, d := range testDirs {
		name := d.Name()
		if !d.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}

		testDir := filepath.Join(sourceDir, name)
		envDir := filepath.Join(testDir, "env")

		files, err := ioutil.ReadDir(envDir)
		require.NoError(t, err, "failed to read dir %s", envDir)

		var envs []v1.Environment
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".yaml" {
				e := v1.Environment{}
				fileName := filepath.Join(envDir, f.Name())
				t.Logf("loading environment %s", fileName)
				data, err := ioutil.ReadFile(fileName)
				require.NoError(t, err, "failed to load environment %s", fileName)

				err = yaml.Unmarshal(data, &e)
				require.NoError(t, err, "failed to unmarshal environment %s", fileName)
				envs = append(envs, e)
			}
		}

		m := upgrader.HelmfileUpgrader{
			Environments: envs,
		}
		requirements, err := m.ExportRequirements()
		require.NoError(t, err, "failed to generate requirements")

		got, err := yaml.Marshal(requirements)
		require.NoError(t, err, "failed to marshal the generated requirements %#v", requirements)

		expectedRequirementsFile := filepath.Join(testDir, "expected-jx-requirements.yml")
		require.FileExists(t, expectedRequirementsFile, "no expected requirements file for test %s", name)

		_, err = config.LoadRequirementsConfigFile(expectedRequirementsFile, true)
		require.NoError(t, err, "failed to validate %s", expectedRequirementsFile)

		want, err := ioutil.ReadFile(expectedRequirementsFile)
		require.NoError(t, err, "failed to load %s", expectedRequirementsFile)

		if diff := cmp.Diff(strings.TrimSpace(string(got)), strings.TrimSpace(string(want))); diff != "" {
			t.Errorf("Unexpected generated requirements file for %s", name)
			t.Log(diff)

			t.Logf("generated requirements for %s:\n", name)
			t.Logf("\n%s\n", string(got))
		}
	}
}
