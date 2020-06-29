package fake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

const (
	/* #nosec */
	v1SecretDataPath = "/v1/secret/data/"
	/* #nosec */
	v1SecretMetadataPath = "/v1/secret/metadata/"
)

// VaultServer a fake vault server for unit testing of vault client operations
type VaultServer struct {
	T    *testing.T
	Data map[string]map[string]interface{}
}

// NewFakeVaultServer creates a fake vault http server for testing
func NewFakeVaultServer(t *testing.T) *httptest.Server {
	fakeVault := &VaultServer{
		T: t,
	}
	server := httptest.NewServer(http.HandlerFunc(fakeVault.Handle))

	t.Logf("using test server on %s", server.URL)
	os.Setenv(vaultapi.EnvVaultAddress, server.URL)
	return server
}

// Handle handles the vault RESTS APIs
func (f *VaultServer) Handle(rw http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	f.T.Logf("invoked %s path: %s", req.Method, path)

	if f.Data == nil {
		f.Data = map[string]map[string]interface{}{}
	}

	if strings.HasPrefix(path, v1SecretDataPath) {
		name := strings.TrimPrefix(path, v1SecretDataPath)
		if req.Method == http.MethodGet {
			data := f.Data[name]
			result := map[string]interface{}{
				"auth": nil,
				"data": map[string]interface{}{
					"data": data,
				},
				"lease_duration": 3600,
				"lease_id":       "",
				"renewable":      false,
			}
			f.returnData(rw, result)
			return
		}
		if req.Method == http.MethodPut || req.Method == http.MethodPost {
			payload := map[string]interface{}{}
			err := json.NewDecoder(req.Body).Decode(&payload)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			data := payload["data"]
			if data != nil {
				m, ok := data.(map[string]interface{})
				if ok {
					f.Data[name] = m
					rw.WriteHeader(http.StatusCreated)
					return
				}
			}
			http.Error(rw, "missing data", http.StatusBadRequest)
		}
	}

	if strings.HasPrefix(path, v1SecretMetadataPath) {
		name := strings.TrimPrefix(path, v1SecretMetadataPath)
		if req.Method == http.MethodGet {
			keys := f.findKeys(name)
			f.T.Logf("found keys %v", keys)

			result := map[string]interface{}{
				"auth": nil,
				"data": map[string]interface{}{
					"keys": keys,
				},
				"lease_duration": 3600,
				"lease_id":       "",
				"renewable":      false,
			}
			f.returnData(rw, result)
			return
		}
	}

	http.Error(rw, jsonErrorMessage("Unsupported Operation"), http.StatusNotFound)
}

func (f *VaultServer) valueKinds(name string) (bool, bool) {
	hasString := false
	hasMap := false
	data := f.Data[name]
	for _, v := range data {
		_, ok := v.(string)
		if ok {
			hasString = true
		} else {
			hasMap = true
		}
	}
	return hasString, hasMap
}

func (f *VaultServer) findKeys(name string) []string {
	answer := []string{}
	nameAndSlash := name + "/"
	for k := range f.Data {
		if strings.HasPrefix(k, nameAndSlash) {
			remaining := strings.TrimPrefix(k, nameAndSlash)
			paths := strings.SplitN(remaining, "/", 2)
			child := paths[0]
			if child != "" {
				hasStrings, hasMaps := f.valueKinds(name + "/" + child)
				if hasStrings {
					answer = append(answer, child)
				}
				if hasMaps {
					answer = append(answer, child+"/")
				}
			}
		}
	}
	return answer
}

func (f *VaultServer) returnData(rw http.ResponseWriter, values interface{}) {
	data, err := json.Marshal(values)
	if err != nil {
		http.Error(rw, jsonErrorMessage(err.Error()), http.StatusInternalServerError)
		return
	}
	_, err = rw.Write(data)
	require.NoError(f.T, err, "failed to write response payload %#v", data)
}

func jsonErrorMessage(message string) string {
	return fmt.Sprintf(`{"error": "%s"}`, message)
}
