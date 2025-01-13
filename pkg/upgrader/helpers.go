package upgrader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func WriteSourceRepositoriesToGitFolder(outDir string, srList *v1.SourceRepositoryList) ([]string, error) {
	exists, err := files.DirExists(outDir)
	if err != nil {
		return nil, fmt.Errorf("failed to check if output dir exists %s: %w", outDir, err)
	}
	if !exists {
		return nil, fmt.Errorf("output dir %s does not exist", outDir)
	}

	for k := range srList.Items {
		sr := srList.Items[k]
		labels := sr.Labels
		if labels != nil {
			if strings.EqualFold(strings.ToLower(labels[kube.LabelGitSync]), "false") {
				continue
			}
		}
		sr.ObjectMeta = EmptyObjectMeta(&sr.ObjectMeta)

		data, err := yaml.Marshal(&sr)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal SourceRepository %s to YAML: %w", sr.Name, err)
		}

		fileName := filepath.Join(outDir, sr.Name+".yaml")
		err = os.WriteFile(fileName, data, files.DefaultFileWritePermissions)
		if err != nil {
			return nil, fmt.Errorf("failed to write file %s for SourceRepository %s to YAML: %w", fileName, sr.Name, err)
		}
	}
	return nil, nil
}

// EmptyObjectMeta lets return a clean ObjectMeta without any cluster or transient specific values
func EmptyObjectMeta(md *metav1.ObjectMeta) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: md.Name,
	}
}
