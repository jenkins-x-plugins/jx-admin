module github.com/jenkins-x/jx-admin

require (
	cloud.google.com/go v0.55.0 // indirect
	github.com/Azure/go-autorest/autorest v0.9.6 // indirect
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/google/go-cmp v0.4.1
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/jenkins-x/go-scm v1.5.177
	github.com/jenkins-x/jx-api v0.0.24
	github.com/jenkins-x/jx-helpers v1.0.86
	github.com/jenkins-x/jx-kube-client v0.0.8
	github.com/jenkins-x/jx-logging v0.0.11
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/onsi/gomega v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/shurcooL/githubv4 v0.0.0-20191102174205-af46314aec7b // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible

	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/kubernetes => k8s.io/kubernetes v1.14.7
)

go 1.13
