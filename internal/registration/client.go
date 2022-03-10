package registration

import (
	context "context"

	pb "github.com/redhatinsights/yggdrasil/protocol"
	grpc "google.golang.org/grpc"
)

//go:generate mockgen -package=registration -destination=mock_registration.go . DispatcherClient
type DispatcherClient interface {
	// Register is called by a worker to indicate it is ready and capable of
	// handling the specified type of work.
	Register(ctx context.Context, in *pb.RegistrationRequest, opts ...grpc.CallOption) (*pb.RegistrationResponse, error)
	// Send is called by a worker to send data to the dispatcher.
	Send(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error)

	GetConfig(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Config, error)
}
