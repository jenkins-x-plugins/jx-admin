package fake

import (
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/vault/client"
)

// FakeClient a fake vault client implementation
type FakeClient struct {
	Data map[string]map[string]interface{}
}

// implements interface
var _ client.Client = (*FakeClient)(nil)

// Read reads data
func (f *FakeClient) Read(name string) (map[string]interface{}, error) {
	if f.Data == nil {
		f.Data = map[string]map[string]interface{}{}
	}
	return f.Data[name], nil
}

// Write writes data
func (f *FakeClient) Write(name string, values map[string]interface{}) error {
	if f.Data == nil {
		f.Data = map[string]map[string]interface{}{}
	}
	f.Data[name] = values
	return nil
}

// String textual info
func (f *FakeClient) String() string {
	return "fake vault"
}
