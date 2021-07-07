package main

import (
	"context"
	"fmt"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/configuration"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/hardware"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/heartbeat"
	deviceos "github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/os"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/registration"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/workload"
	"net"
	"os"
	"path"
	"time"

	"git.sr.ht/~spc/go-log"

	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

var yggdDispatchSocketAddr string

func main() {
	logLevel, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		logLevel = "ERROR"
	}
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		level = log.LevelError
	}
	log.SetLevel(level)
	// Get initialization values from the environment.
	yggdDispatchSocketAddr, ok = os.LookupEnv("YGG_SOCKET_ADDR")
	if !ok {
		log.Fatal("Missing YGG_SOCKET_ADDR environment variable")
	}
	baseConfigDir, ok := os.LookupEnv("BASE_CONFIG_DIR")
	if !ok {
		log.Fatal("Missing BASE_CONFIG_DIR environment variable")
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
	configDir := path.Join(baseConfigDir, "device")
	log.Infof("Config dir: %s", configDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatal(fmt.Errorf("cannot create directory: %w", err))
	}
	configManager := configuration.NewConfigurationManager(configDir)
	hw := hardware.Hardware{}
	deviceOs := deviceos.OS{}
	hbs := heartbeat.NewHeartbeatService(c, configManager)
	configManager.RegisterObserver(hbs)
	reg := registration.NewRegistration(&hw, &deviceOs, c)
	wl, err := workload.NewWorkload(configDir)
	if err != nil {
		log.Fatal(err)
	}
	configManager.RegisterObserver(wl)
	s := grpc.NewServer()
	pb.RegisterWorkerServer(s, NewDeviceServer(configManager))
	if configManager.IsInitialConfig() {
		err := reg.RegisterDevice()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		hbs.Start()
	}

	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
