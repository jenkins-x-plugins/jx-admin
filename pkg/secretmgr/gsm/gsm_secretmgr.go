package gsm

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// GoogleSecretManager uses a Kubernetes Secret
type GoogleSecretManager struct {
	SecretName string
}

// NewGoogleSecretManager uses a Kubernetes Secret to manage secrets
func NewGoogleSecretManager(requirements *config.RequirementsConfig) (secretmgr.SecretManager, error) {
	clusterName := requirements.Cluster.ClusterName
	if clusterName == "" {
		return nil, fmt.Errorf("no cluster.clusterName in the requirements")
	}
	secretName := fmt.Sprintf("%s-boot-secret", clusterName)

	// TODO should we verify we have gcloud beta setup?

	sm := &GoogleSecretManager{SecretName: secretName}

	return sm, nil
}

// UpsertSecrets upserts the secrets
func (f *GoogleSecretManager) UpsertSecrets(callback secretmgr.SecretCallback, defaultYaml string) error {
	err := f.ensureSecretExists()
	if err != nil {
		return err
	}

	secretYaml, err := f.getSecret()
	if err != nil {
		// lets assume its the first version
		log.Logger().Debugf("ignoring error %s", err.Error())
	}

	if secretYaml == "" {
		secretYaml = defaultYaml
	}

	updatedYaml, err := callback(secretYaml)
	if err != nil {
		return err
	}
	if updatedYaml != secretYaml {
		return f.updateSecretYaml(updatedYaml)
	}
	return nil
}

func (f *GoogleSecretManager) Kind() string {
	return secretmgr.KindGoogleSecretManager
}

func (f *GoogleSecretManager) String() string {
	return fmt.Sprintf("Google Secret Manager for secret %s", f.SecretName)
}

func (f *GoogleSecretManager) getSecret() (string, error) {
	c := util.Command{
		Name: "gcloud",
		Args: []string{"beta", "secrets", "versions", "access", "latest", "--secret=" + f.SecretName, "-q"},
	}
	log.Logger().Debugf("running gcloud %s", strings.Join(c.Args, " "))

	text, err := c.RunWithoutRetry()
	if err != nil {
		return "", err
	}
	return text, nil
}

func (f *GoogleSecretManager) secretExists() bool {
	c := util.Command{
		Name: "gcloud",
		Args: []string{"beta", "secrets", "list", "--filter=" + f.SecretName},
	}

	log.Logger().Debugf("running gcloud %s", strings.Join(c.Args, " "))
	text, err := c.RunWithoutRetry()
	if err != nil {
		// lets assume it does not exist yet
		return false
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	// ignore the headers
	lines = lines[1:]
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == f.SecretName {
			return true
		}
		log.Logger().Infof("unknown secret name '%s'", fields[0])
	}
	return false
}

func (f *GoogleSecretManager) updateSecretYaml(newYaml string) error {
	tmpFile, err := ioutil.TempFile("", "gsm-secret-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}
	fileName := tmpFile.Name()
	defer os.Remove(fileName)

	err = ioutil.WriteFile(fileName, []byte(newYaml), util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save secrets to temp file %s", fileName)
	}

	c := util.Command{
		Name: "gcloud",
		Args: []string{"beta", "secrets", "versions", "add", f.SecretName, "--data-file", fileName},
	}
	log.Logger().Debugf("running gcloud %s", strings.Join(c.Args, " "))

	_, err = c.RunWithoutRetry()
	return err
}

func (f *GoogleSecretManager) ensureSecretExists() error {
	exists := f.secretExists()
	if exists {
		return nil
	}
	c := util.Command{
		Name: "gcloud",
		Args: []string{"beta", "secrets", "create", f.SecretName, "--replication-policy", "automatic"},
	}
	log.Logger().Debugf("running gcloud %s", strings.Join(c.Args, " "))
	_, err := c.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to ensure the google secret %s exists", f.SecretName)
	}
	return nil
}
