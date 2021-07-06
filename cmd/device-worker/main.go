package main

import (
	"context"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/configuration"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/heartbeat"
	"net"
	"os"
	"time"

	"git.sr.ht/~spc/go-log"

	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

var yggdDispatchSocketAddr string

func main() {
	log.SetLevel(log.LevelInfo)
	// Get initialization values from the environment.
	var ok bool
	yggdDispatchSocketAddr, ok = os.LookupEnv("YGG_SOCKET_ADDR")
	if !ok {
		log.Fatal("Missing YGG_SOCKET_ADDR environment variable")
	}

	// Dial the dispatcher on its well-known address.
	conn, err := grpc.Dial(yggdDispatchSocketAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a dispatcher client
	c := pb.NewDispatcherClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Register as a handler of the "device" type.
	r, err := c.Register(ctx, &pb.RegistrationRequest{Handler: "device", Pid: int64(os.Getpid())})
	if err != nil {
		log.Fatal(err)
	}
	if !r.GetRegistered() {
		log.Fatalf("handler registration failed: %v", err)
	}

	// Listen on the provided socket address.
	l, err := net.Listen("unix", r.GetAddress())
	if err != nil {
		log.Fatal(err)
	}

	// Register as a Worker service with gRPC and start accepting connections.
	configManager := configuration.NewConfigurationManager()
	hbs := heartbeat.NewHeartbeatService(c, configManager)
	configManager.RegisterObserver(hbs)

	s := grpc.NewServer()
	pb.RegisterWorkerServer(s, NewDeviceServer(configManager))

	hbs.Start()

	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
