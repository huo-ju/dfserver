package worker

import (
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
)

type ProcessWorkerIface interface {
	Name() string
	Work(lastoutput *dfpb.Output, settings []byte) (bool, error)
}
