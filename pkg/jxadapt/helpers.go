package jxadapt

import (
	"os"

	"github.com/jenkins-x/jx-remote/pkg/fakes/fakeclientsfactory"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
)

// JXAdapter an adapter between new clean code and the classic CommonOptions abstractions in jx
// to allow us to move new code away from CommonOptions while reusing existing code
type JXAdapter struct {
	JXFactory jxfactory.Factory
	Gitter    gits.Gitter
	BatchMode bool
}

// NewJXAdapter creates a new adapter
func NewJXAdapter(f jxfactory.Factory, gitter gits.Gitter, batch bool) *JXAdapter {
	if f == nil {
		f = jxfactory.NewFactory()
	}
	return &JXAdapter{
		JXFactory: f,
		Gitter:    gitter,
		BatchMode: batch,
	}
}

// NewCommonOptions creates a CommonOptions that can be used for integrating with jx code which uses it
func (a *JXAdapter) NewCommonOptions() *opts.CommonOptions {
	f := clients.NewUsingFactory(a.JXFactory)
	co := opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)

	cf := fakeclientsfactory.NewFakeFactory(a.JXFactory, f)
	co.SetFactory(cf)

	kubeClient, ns, err := a.JXFactory.CreateKubeClient()
	if err == nil {
		co.SetDevNamespace(ns)
		co.SetKubeClient(kubeClient)
	}
	jxClient, _, err := a.JXFactory.CreateJXClient()
	if err == nil {
		co.SetJxClient(jxClient)
	}
	co.SetKube(a.JXFactory.KubeConfig())

	co.BatchMode = a.BatchMode
	co.SetGit(a.Gitter)
	return co
}
