package worker

import (
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
)

type ProcessWorkerIface interface {
	Name() string
	Work(outputList []*dfpb.Output, lastinput *dfpb.Input, settings []byte) (bool, error)
}
