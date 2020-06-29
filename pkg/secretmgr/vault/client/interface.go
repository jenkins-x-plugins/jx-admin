package client

// Client interface for vault clients
type Client interface {

	// Read reads a tree of values from the vault
	Read(name string) (map[string]interface{}, error)

	// Write writes the given tree of values to the given name
	Write(name string, values map[string]interface{}) error

	// String returns the textual representation
	String() string
}
