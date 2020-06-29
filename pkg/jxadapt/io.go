package jxadapt

import (
	"os"

	"github.com/jenkins-x/jx/pkg/util"
)

// ToIOHandles handles lazily creating the IO handles if they are nil
func ToIOHandles(handles *util.IOFileHandles) util.IOFileHandles {
	if handles == nil {
		handles = &util.IOFileHandles{
			Err: os.Stderr,
			In:  os.Stdin,
			Out: os.Stdout,
		}
	}
	return *handles
}
