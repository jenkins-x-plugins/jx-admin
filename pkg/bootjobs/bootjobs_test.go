package bootjobs

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSelectOperatorNamespace_UsesUserProvidedNamespaceFirst(t *testing.T) {
	assert.Equal(t, []string{"my-custom-namespace", "jx", "jx-git-operator"}, selectOperatorNamespace("my-custom-namespace"))
}

func TestSelectOperatorNamespace_DoesNotAppendEmptyNamespace(t *testing.T) {
	assert.Equal(t, []string{"jx", "jx-git-operator"}, selectOperatorNamespace(""))
}
