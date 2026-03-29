// Package proto holds protobuf source definitions for all downstream services.
// Run the command below to regenerate Go code from .proto files.
// Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
//
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative user/user.proto
package proto
