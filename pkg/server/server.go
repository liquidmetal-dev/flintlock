package server

import (
	mvmv1 "github.com/weaveworks/reignite/api/services/microvm/v1alpha1"
)

// NewServer creates a new server instance.
// NOTE: this is an unimplemented server at present.
func NewServer() mvmv1.MicroVMServer {
	return &server{}
}

type server struct {
	mvmv1.UnimplementedMicroVMServer
}
