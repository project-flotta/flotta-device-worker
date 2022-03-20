package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/project-flotta/flotta-device-worker/internal/logs"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"

	configuration2 "github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer"
	hardware2 "github.com/project-flotta/flotta-device-worker/internal/hardware"
	heartbeat2 "github.com/project-flotta/flotta-device-worker/internal/heartbeat"
	os2 "github.com/project-flotta/flotta-device-worker/internal/os"
	registration2 "github.com/project-flotta/flotta-device-worker/internal/registration"
	"github.com/project-flotta/flotta-device-worker/internal/server"
	workload2 "github.com/project-flotta/flotta-device-worker/internal/workload"

	"net"
	"os"
	"path"
	"time"

	"git.sr.ht/~spc/go-log"

	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

var yggdDispatchSocketAddr string
var gracefulRebootChannel chan struct{}

const (
	defaultDataDir = "/var/local/yggdrasil"
)

func main() {
	log.SetFlags(0) // No datatime, is already done on yggradsil server

	logLevel, ok := os.LookupEnv("YGG_LOG_LEVEL")
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
		log.Fatal("missing YGG_SOCKET_ADDR environment variable")
	}

	baseDataDir, ok := os.LookupEnv("YGG_CONFIG_DIR")
	if !ok {
		log.Warnf("missing BASE_DATA_DIR environment variable. Using default: %s", defaultDataDir)
		baseDataDir = defaultDataDir
	}

	// Dial the dispatcher on its well-known address.
	conn, err := grpc.Dial(yggdDispatchSocketAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a dispatcher client
	dispatcherClient := pb.NewDispatcherClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Register as a handler of the "device" type.
	r, err := dispatcherClient.Register(ctx, &pb.RegistrationRequest{Handler: "device", Pid: int64(os.Getpid())})
	if err != nil {
		log.Fatal(err)
	}
	if !r.GetRegistered() {
		log.Fatal("yggdrasil registration failed")
	}

	// Listen on the provided socket address.
	l, err := net.Listen("unix", r.GetAddress())
	if err != nil {
		log.Fatalf("cannot start listening on %s err: %v", r.GetAddress(), err)
	}

	// Register as a Worker service with gRPC and start accepting connections.
	dataDir := path.Join(baseDataDir, "device")
	log.Infof("Data directory: %s", dataDir)
	/* #nosec */
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal(fmt.Errorf("cannot create directory: %w", err))
	}
	deviceId, ok := os.LookupEnv("YGG_CLIENT_ID")
	if !ok {
		log.Warn("DEVICE_ID environment variable has not been set")
		deviceId = "unknown"
	}
	configManager := configuration2.NewConfigurationManager(dataDir)

	// --- Client metrics configuration ---
	metricsStore, err := metrics.NewTSDB(dataDir)
	if err != nil {
		log.Fatalf("cannot initialize TSDB, err: %v", err)
	}
	configManager.RegisterObserver(metricsStore)
	// Metrics Daemon
	metricsDaemon := metrics.NewMetricsDaemon(metricsStore)

	workloadMetricWatcher := metrics.NewWorkloadMetrics(metricsDaemon)
	configManager.RegisterObserver(workloadMetricWatcher)

	systemMetricsWatcher, err := metrics.NewSystemMetrics(metricsDaemon)
	if err != nil {
		log.Fatalf("cannot initialize system metrics. DeviceID: %s; err: %v", deviceId, err)
	}
	configManager.RegisterObserver(systemMetricsWatcher)

	wl, err := workload2.NewWorkloadManager(dataDir, deviceId)
	if err != nil {
		log.Fatalf("cannot start Workload Manager. DeviceID: %s; err: %v", deviceId, err)
	}

	logsWrapper := logs.NewWorkloadsLogsTarget(wl)
	configManager.RegisterObserver(logsWrapper)
	wl.RegisterObserver(logsWrapper)
	configManager.RegisterObserver(wl)
	wl.RegisterObserver(workloadMetricWatcher)

	remoteWrite := metrics.NewRemoteWrite(dataDir, deviceId, metricsStore)
	configManager.RegisterObserver(remoteWrite)

	hw := hardware2.Hardware{}

	gracefulRebootChannel = make(chan struct{})
	deviceOs := os2.NewOS(gracefulRebootChannel, os2.NewOsExecCommands())
	configManager.RegisterObserver(deviceOs)

	dataMonitor := datatransfer.NewMonitor(wl, configManager)
	wl.RegisterObserver(dataMonitor)
	configManager.RegisterObserver(dataMonitor)
	dataMonitor.Start()

	if err != nil {
		log.Fatalf("cannot start metrics store. DeviceID: %s; err: %v", deviceId, err)
	}

	reg, err := registration2.NewRegistration(deviceId, &hw, dispatcherClient, configManager, wl)
	if err != nil {
		log.Fatalf("cannot start registration process:  DeviceID: %s; err: %v", deviceId, err)
	}

	hbs := heartbeat2.NewHeartbeatService(dispatcherClient, configManager, wl, &hw, dataMonitor, deviceOs, reg)
	configManager.RegisterObserver(hbs)

	reg.DeregisterLater(
		wl,
		configManager,
		hbs,
		dataMonitor,
		systemMetricsWatcher,
		metricsStore,
	)

	s := grpc.NewServer()
	pb.RegisterWorkerServer(s, server.NewDeviceServer(configManager, reg))
	if !configManager.IsInitialConfig() {
		hbs.Start()
	} else {
		reg.RegisterDevice()
	}

	setupSignalHandler(metricsStore)

	go listenStartGracefulRebootChannel(wl, dataMonitor, systemMetricsWatcher, metricsStore, hbs,
		gracefulRebootChannel, deviceOs)

	if err := s.Serve(l); err != nil {
		log.Fatalf("cannot start worker server, err: %v", err)
	}

}

func listenStartGracefulRebootChannel(wl *workload2.WorkloadManager, dataMonitor *datatransfer.Monitor,
	systemMetricsWatcher *metrics.SystemMetrics, metricsStore *metrics.TSDB, hbs *heartbeat2.Heartbeat,
	gracefulRebootChannel chan struct{}, deviceOs *os2.OS) {
	// listen to the channel for getting StartGracefulReboot signal
	for {
		<-gracefulRebootChannel
		log.Info("A graceful reboot request was received")
		if err := wl.StopWorkloads(); err != nil {
			log.Fatalf("cannot graceful reboot the workloads: %v", err)
		}

		if err := dataMonitor.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot dataMonitor: %v", err)
		}

		if err := systemMetricsWatcher.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot systemMetricsWatcher: %v", err)
		}

		if err := metricsStore.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot metricsStore: %v", err)
		}

		if err := hbs.Deregister(); err != nil {
			log.Fatalf("cannot graceful reboot the heartbeat service: %v", err)
		}

		deviceOs.GracefulRebootCompletionChannel <- struct{}{}
	}
}

func setupSignalHandler(metricsStore *metrics.TSDB) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-c
		log.Infof("Got %s signal. Aborting...\n", sig)
		closeComponents(metricsStore)
		os.Exit(0)
	}()
}

func closeComponents(metricsStore *metrics.TSDB) {
	if err := metricsStore.Close(); err != nil {
		log.Error(err)
	}
}
