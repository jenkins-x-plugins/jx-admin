package secretmgr

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx-remote/pkg/githelpers"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

var expectedSecretPaths = []string{
	"secrets.adminUser.username",
	"secrets.adminUser.password",
	"secrets.hmacToken",
	"secrets.pipelineUser.username",
	"secrets.pipelineUser.email",
	"secrets.pipelineUser.token",
}

// VerifyBootSecrets verifies the boot secrets
func VerifyBootSecrets(secretsYAML string) error {
	data := map[string]interface{}{}

	err := yaml.Unmarshal([]byte(secretsYAML), &data)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal secrets YAML")
	}

	// simple validation for now, using presence of a string value
	for _, path := range expectedSecretPaths {
		value := util.GetMapValueAsStringViaPath(data, path)
		if value == "" {
			return errors.Errorf("missing secret entry: %s", path)
		}
	}
	return nil
}

// ToSecretsYAML converts the data to secrets YAML
func ToSecretsYAML(values map[string]interface{}) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	secrets := map[string]interface{}{
		"secrets": values,
	}
	data, err := yaml.Marshal(secrets)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal data to YAML")
	}
	return string(data), nil
}

// PipelineUserTokenFromSecretsYAML returns the pipeline user and token from the Secrets YAML
func PipelineUserTokenFromSecretsYAML(data []byte, message string) (string, string, error) {
	yamlData := map[string]interface{}{}
	err := yaml.Unmarshal(data, &yamlData)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to parse %s", message)
	}

	username := util.GetMapValueAsStringViaPath(yamlData, "secrets.pipelineUser.username")
	if username == "" {
		log.Logger().Warnf("missing secret: secrets.pipelineUser.username")
		return "", "", nil
	}
	token := util.GetMapValueAsStringViaPath(yamlData, "secrets.pipelineUser.token")
	if token == "" {
		log.Logger().Warnf("missing secret: secrets.pipelineUser.token")
		return "", "", nil
	}
	return username, token, nil
}

// AddUserTokenToGitURLFromSecretsYAML adds the user/token to the git URL if the secrets YAML is not empty
func AddUserTokenToGitURLFromSecretsYAML(gitURL string, secretsYAML string) (string, error) {
	user, token, err := PipelineUserTokenFromSecretsYAML([]byte(secretsYAML), "secrets YAML")
	if err != nil {
		return "", errors.Wrap(err, "failed to find pipeline git user and token from secrets YAML")
	}
	if user == "" {
		return "", fmt.Errorf("missing secrets.pipelineUser.username")
	}
	if token == "" {
		return "", fmt.Errorf("missing secrets.pipelineUser.token")
	}
	gitURL, err = githelpers.AddUserTokenToURLIfRequired(gitURL, user, token)
	if err != nil {
		return "", errors.Wrapf(err, "failed to add git user and token into give URL %s", gitURL)
	}
	return gitURL, nil
}

// RemoveMapEmptyValues recursively removes all empty string or nil entries
func RemoveMapEmptyValues(m map[string]interface{}) {
	for k, v := range m {
		if v == nil || v == "" {
			delete(m, k)
		}
		childMap, ok := v.(map[string]interface{})
		if ok {
			RemoveMapEmptyValues(childMap)
		}
	}
}

// UnmarshalSecretsYAML unmarshals the given Secrets YAML
func UnmarshalSecretsYAML(secretsYaml string) (map[string]interface{}, error) {
	data := map[string]interface{}{}

	if strings.TrimSpace(secretsYaml) != "" {
		err := yaml.Unmarshal([]byte(secretsYaml), &data)
		if err != nil {
			return data, errors.Wrap(err, "failed to unmarshal YAML")
		}
	}
	existing := map[string]interface{}{}
	existingSecrets := data["secrets"]
	existingSecretsMap, ok := existingSecrets.(map[string]interface{})
	if ok {
		for k, v := range existingSecretsMap {
			if v != nil && v != "" {
				existing[k] = v
			}
		}
	}
	RemoveMapEmptyValues(existing)
	return existing, nil
}
