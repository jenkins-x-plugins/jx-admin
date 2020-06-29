package secretmgr

type SecretCallback func(secretYaml string) (string, error)

type SecretManager interface {

	// UpsertSecrets inserts or updates the secrets using some kind of storage
	// with the callback taking the current or default secrets, invoking the callback to modify them
	// then storing them in a cloud secret manager, local kubernetes Secret or vault etc.
	UpsertSecrets(callback SecretCallback, defaultYaml string) error

	// Kind returns the kind of the Secret Manager
	Kind() string

	// String returns the string description of the secrets manager
	String() string
}
