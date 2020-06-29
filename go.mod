module github.com/jenkins-x/jx-remote

require (
	github.com/banzaicloud/bank-vaults v0.0.0-20190508130850-5673d28c46bd
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/cli/cli v0.6.2
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/genkiroid/cert v0.0.0-20191007122723-897560fbbe50
	github.com/google/go-cmp v0.3.0
	github.com/google/uuid v1.1.1
	github.com/hashicorp/vault v1.1.2
	github.com/heptio/sonobuoy v0.16.0
	github.com/jenkins-x/go-scm v1.5.136
	github.com/jenkins-x/golang-jenkins v0.0.0-20180919102630-65b83ad42314
	github.com/jenkins-x/helmboot v0.0.95
	github.com/jenkins-x/jx v1.3.981-0.20200625050556-5ccee8e660bc
	github.com/jetstack/cert-manager v0.5.2
	github.com/knative/serving v0.7.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/petergtz/pegomock v2.7.0+incompatible
	github.com/pkg/errors v0.8.1
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	github.com/tektoncd/pipeline v0.8.0
	gopkg.in/yaml.v3 v3.0.0-20200121175148-a6ecf24a6d71
	k8s.io/api v0.0.0-20190718183219-b59d8169aab5
	k8s.io/apiextensions-apiserver v0.0.0-20190718185103-d1ef975d28ce
	k8s.io/apimachinery v0.0.0-20190703205208-4cfb76a8bf76
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/kubernetes v1.11.3
	k8s.io/metrics v0.0.0-20180620010437-b11cf31b380b
	k8s.io/test-infra v0.0.0-20190131093439-a22cef183a8f
	sigs.k8s.io/yaml v1.1.0

)

replace k8s.io/api => k8s.io/api v0.0.0-20190528110122-9ad12a4af326

replace k8s.io/metrics => k8s.io/metrics v0.0.0-20181128195641-3954d62a524d

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221084156-01f179d85dbc

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190528110200-4f3abb12cae2

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190528110544-fa58353d80f3

replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999

replace github.com/sirupsen/logrus => github.com/jtnord/logrus v1.4.2-0.20190423161236-606ffcaf8f5d

replace github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v21.1.0+incompatible

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v10.14.0+incompatible

replace github.com/banzaicloud/bank-vaults => github.com/banzaicloud/bank-vaults v0.0.0-20190508130850-5673d28c46bd

go 1.13
