package operator_test

import (
	"testing"

	"github.com/jenkins-x-plugins/jx-admin/pkg/cmd/operator"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner/fakerunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/helmer"
	"github.com/stretchr/testify/require"
)

func TestOperator(t *testing.T) {
	testCases := []struct {
		description  string
		userName     string
		skipcreatens bool
		expectError  bool
	}{
		{
			userName:     "fakegitusername",
			skipcreatens: false,
		},
		{
			userName:     "myname@cheese.com",
			expectError:  true,
			skipcreatens: false,
		},
		{
			userName:     "fakegitusernamenons",
			skipcreatens: true,
		},
	}

	for _, tc := range testCases {
		runner := &fakerunner.FakeRunner{}

		_, o := operator.NewCmdOperator()
		o.CommandRunner = runner.Run
		o.HelmBin = "helm"
		o.GitUserName = tc.userName
		o.GitToken = "fakegittoken"
		o.GitURL = "https://github.com/jx3-gitops-repositories/jx3-kubernetes"
		o.Helmer = helmer.NewFakeHelmer()
		o.NoLog = true
		if tc.skipcreatens {
			o.SkipNamespaceCreation = true
		}

		err := o.Run()
		if tc.expectError {
			require.Error(t, err, "expected error on run for user %s", tc.userName)
			t.Logf("got expected error for username %s error %s\n", tc.userName, err.Error())
		} else {
			require.NoError(t, err, "failed to run for user %s", tc.userName)
		}
	}
}
