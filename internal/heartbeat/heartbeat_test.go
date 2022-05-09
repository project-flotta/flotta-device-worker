package heartbeat_test

import (
	"context"
	"fmt"
	"net"

	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-device-worker/internal/ansible"
	os2 "github.com/project-flotta/flotta-device-worker/internal/os"
	"github.com/project-flotta/flotta-device-worker/internal/registration"

	"github.com/golang/mock/gomock"
	"github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer"
	"github.com/project-flotta/flotta-device-worker/internal/hardware"
	"github.com/project-flotta/flotta-device-worker/internal/heartbeat"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-device-worker/internal/workload/api"
	"github.com/project-flotta/flotta-operator/models"
	"google.golang.org/grpc"

	pb "github.com/redhatinsights/yggdrasil/protocol"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Heartbeat", func() {

	var (
		datadir        = "/tmp"
		ansibleDir     = "/tmp/ansible_test"
		mockCtrl       *gomock.Controller
		wkManager      *workload.WorkloadManager
		configManager  *configuration.Manager
		ansibleManager *ansible.Manager
		wkwMock        *workload.MockWorkloadWrapper
		hwMock         *hardware.MockHardware
		monitor        = &datatransfer.Monitor{}
		hb             = &heartbeat.Heartbeat{}
		err            error
		client         Dispatcher
		deviceOs       *os2.OS
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)
		wkwMock.EXPECT().Init().Return(nil).AnyTimes()
		wkwMock.EXPECT().PersistConfiguration().AnyTimes()

		regMock := registration.NewMockRegistrationWrapper(mockCtrl)
		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock, "device-id-123")
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")

		hwMock = hardware.NewMockHardware(mockCtrl)

		configManager = configuration.NewConfigurationManager(datadir)

		client = Dispatcher{}
		gracefulRebootChannel := make(chan struct{})
		osExecCommands := os2.NewOsExecCommands()
		deviceOs = os2.NewOS(gracefulRebootChannel, osExecCommands)

		ansibleManager, err = ansible.NewAnsibleManager(&client, ansibleDir)
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Ansible Manager")

		hb = heartbeat.NewHeartbeatService(&client,
			configManager,
			wkManager,
			hwMock,
			monitor,
			deviceOs,
			regMock)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("HeartBeatData test", func() {
		It("Report empty workloads an up status", func() {
			//given
			wkwMock.EXPECT().List().Times(1)
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hwMock, monitor, deviceOs)

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))
			Expect(heartbeatInfo.Workloads).To(BeEmpty())
		})

		It("Report workload correctly", func() {
			//given
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hwMock, monitor, deviceOs)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{{
				Id:     "test",
				Name:   "test",
				Status: "Running",
			}}, nil).AnyTimes()

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))

			// Workload checks
			Expect(heartbeatInfo.Workloads).To(HaveLen(1))
			Expect(heartbeatInfo.Workloads[0].Name).To(Equal("test"))
			Expect(heartbeatInfo.Workloads[0].Status).To(Equal("Running"))
		})

		It("Report ansible events correctly", func() {
			//given
			ansibleErrorMsg := "test playbook error string"
			ansibleEventReason := "Failed"
			ansibleManager.AddToEventQueue(&models.EventInfo{
				Message: ansibleErrorMsg,
				Reason:  ansibleEventReason,
				Type:    models.EventInfoTypeWarn,
			})
			//given
			wkwMock.EXPECT().List().Times(1)
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hwMock, monitor, deviceOs)

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then

			// Workload checks
			Expect(heartbeatInfo.Status).To(Equal("up"))
			Expect(heartbeatInfo.Workloads).To(BeEmpty())

			// Ansible checks
			Expect(heartbeatInfo.Events).To(HaveLen(1))
			Expect(heartbeatInfo.Events[0].Message).To(Equal(ansibleErrorMsg))
			Expect(heartbeatInfo.Events[0].Reason).To(Equal(ansibleEventReason))
		})

		It("Cannot retrieve the list of workloads", func() {
			//given
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hwMock, monitor, deviceOs)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{}, fmt.Errorf("invalid list")).AnyTimes()

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))
			Expect(heartbeatInfo.Workloads).To(HaveLen(0))
		})

		It("Report workload hw delta enable", func() {
			//given
			var m models.HardwareInfo
			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Scope = heartbeat.ScopeDelta
			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Include = true
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hwMock, monitor, deviceOs)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{{
				Id:     "test",
				Name:   "test",
				Status: "Running",
			}}, nil).AnyTimes()

			hwMock.EXPECT().GetHardwareInformation().Return(&models.HardwareInfo{
				Hostname: "localhost",
				Interfaces: []*models.Interface{{
					IPV4Addresses: []string{"127.0.0.1", "0.0.0.0"},
				}},
				CPU:          &models.CPU{Architecture: "TestArchi", ModelName: "ModelTest"},
				SystemVendor: &models.SystemVendor{Manufacturer: "ManufacturerTest", ProductName: "ProductTest", SerialNumber: "SerialTest"},
			}, nil)

			hwMock.EXPECT().GetMutableHardwareInfoDelta(gomock.AssignableToTypeOf(m), gomock.AssignableToTypeOf(m)).DoAndReturn(
				func(hardwareMutableInfoPrevious models.HardwareInfo, hardwareMutableInfoNew models.HardwareInfo) *models.HardwareInfo {
					return hardware.GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious, hardwareMutableInfoNew)
				}).AnyTimes()

			hwMock.EXPECT().CreateHardwareMutableInformation().Return(&models.HardwareInfo{
				Hostname: "localhost",
				Interfaces: []*models.Interface{{
					IPV4Addresses: []string{"127.0.0.1", "0.0.0.0"},
				}},
			}).Times(4)

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))

			// Workload checks
			Expect(heartbeatInfo.Workloads).To(HaveLen(1))
			Expect(heartbeatInfo.Workloads[0].Name).To(Equal("test"))
			Expect(heartbeatInfo.Workloads[0].Status).To(Equal("Running"))

			// Hardware checks first time
			Expect(heartbeatInfo.Hardware.CPU).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal("localhost"))
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(Not(BeNil()))

			// Hardware checks delta time
			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(BeEmpty())
			Expect(heartbeatInfo.Hardware.Interfaces).To(BeNil())
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

			// Hardware checks delta time hostname change
			hwMock.EXPECT().CreateHardwareMutableInformation().Return(&models.HardwareInfo{
				Hostname: "localhostNEW",
				Interfaces: []*models.Interface{{
					IPV4Addresses: []string{"127.0.0.1", "0.0.0.0"},
				}},
			}).Times(2)
			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal("localhostNEW"))
			Expect(heartbeatInfo.Hardware.Interfaces).To(BeNil())
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

			// Hardware checks delta time interface	 change
			hwMock.EXPECT().CreateHardwareMutableInformation().Return(&models.HardwareInfo{
				Hostname: "localhostNEW",
				Interfaces: []*models.Interface{{
					IPV4Addresses: []string{"127.0.0.1", "0.0.0.0"},
					IPV6Addresses: []string{"f8:75:a4:a4:00:fe"},
				}},
			}).Times(4)

			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(BeEmpty())
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

			// Hardware checks delta time
			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(BeEmpty())
			Expect(heartbeatInfo.Hardware.Interfaces).To(BeNil())
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

			// Hardware checks delta time hostname and interface change
			hwMock.EXPECT().CreateHardwareMutableInformation().Return(&models.HardwareInfo{
				Hostname: "localhostFINAL",
				Interfaces: []*models.Interface{{
					IPV4Addresses: []string{"127.0.0.1", "0.0.0.0", "10.0.0.1"},
					IPV6Addresses: []string{"f8:75:a4:a4:00:fe"},
				}},
			}).Times(2)
			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal("localhostFINAL"))
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

		})

		It("Report workload hw delta disable", func() {
			//given
			hostname := "localhost"
			interfaces := []*models.Interface{{
				IPV4Addresses: []string{"127.0.0.1", "0.0.0.0"},
			}}

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{{
				Id:     "test",
				Name:   "test",
				Status: "Running",
			}}, nil).AnyTimes()

			hwMock.EXPECT().GetHardwareInformation().Return(&models.HardwareInfo{
				Hostname:     hostname,
				Interfaces:   interfaces,
				CPU:          &models.CPU{Architecture: "TestArchi", ModelName: "ModelTest"},
				SystemVendor: &models.SystemVendor{Manufacturer: "ManufacturerTest", ProductName: "ProductTest", SerialNumber: "SerialTest"},
			}, nil)

			hwMock.EXPECT().CreateHardwareMutableInformation().Return(&models.HardwareInfo{
				Hostname:   hostname,
				Interfaces: interfaces,
			}).AnyTimes()

			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Scope = heartbeat.ScopeFull
			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Include = true
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hwMock, monitor, deviceOs)

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			// Hardware checks first time
			Expect(heartbeatInfo.Hardware.CPU).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal(hostname))
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(Not(BeNil()))

			// Hardware checks second time: only mutable info
			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal(hostname))
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

		})

		It("Report workload hw info disable", func() {
			//given
			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{{
				Id:     "test",
				Name:   "test",
				Status: "Running",
			}}, nil).AnyTimes()

			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Include = false
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hwMock, monitor, deviceOs)

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			// Hardware checks first time
			Expect(heartbeatInfo.Hardware).To(BeNil())
		})
	})

	Context("Start", func() {
		It("Ticker is stopped if it's not started", func() {
			//given
			Expect(hb.HasStarted()).To(BeFalse(), "Ticker is initialized when it shouldn't")

			// when
			hb.Start()

			//then
			Expect(hb.HasStarted()).To(BeTrue())
		})
	})

	Context("Update", func() {

		BeforeEach(func() {
			wkwMock.EXPECT().List().AnyTimes()
		})

		It("Ticker is created", func() {

			//given
			Expect(hb.HasStarted()).To(BeFalse(), "Ticker is initialized when it shouldn't")

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     []*models.Workload{},
			}

			// when
			err := hb.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(hb.HasStarted()).To(BeTrue())
		})
		It("Ticker not created on invalid config", func() {

			// given
			Expect(hb.HasStarted()).To(BeFalse(), "Ticker is initialized when it shouldn't")

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{},
				DeviceID:      "",
				Version:       "",
				Workloads:     []*models.Workload{},
			}

			// when
			err := hb.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(hb.HasStarted()).To(BeTrue())
		})

	})

})

// We keep the latest send data to make sure that we validate the data sent to
// the operator without sent at all
type Dispatcher struct {
	latestData *pb.Data
}

func (d *Dispatcher) Send(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
	d.latestData = in
	return nil, nil
}

func (d *Dispatcher) Register(ctx context.Context, in *pb.RegistrationRequest, opts ...grpc.CallOption) (*pb.RegistrationResponse, error) {
	return nil, nil
}

func (d *Dispatcher) GetConfig(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Config, error) {
	return nil, nil
}

func NewFilledInterfaceMock(mtu int, name string, macAddr string, flags net.Flags, addrs []string, isPhysical bool, isBonding bool, isVlan bool, speedMbps int64) *util.MockInterface {
	hwAddr, _ := net.ParseMAC(macAddr)
	ret := util.MockInterface{}
	ret.On("IsPhysical").Return(isPhysical)
	if isPhysical || isBonding || isVlan {
		ret.On("Name").Return(name)
		ret.On("MTU").Return(mtu)
		ret.On("HardwareAddr").Return(hwAddr)
		ret.On("Flags").Return(flags)
		ret.On("Addrs").Return(toAddresses(addrs), nil)
		ret.On("SpeedMbps").Return(speedMbps)
	}
	if !isPhysical {
		ret.On("IsBonding").Return(isBonding)
	}
	if !(isPhysical || isBonding) {
		ret.On("IsVlan").Return(isVlan)
	}

	return &ret
}

func toAddresses(addrs []string) []net.Addr {
	ret := make([]net.Addr, 0)
	for _, a := range addrs {
		ret = append(ret, str2Addr(a))
	}
	return ret
}

func str2Addr(addrStr string) net.Addr {
	ip, ipnet, err := net.ParseCIDR(addrStr)
	if err != nil {
		return &net.IPNet{}
	}
	return &net.IPNet{IP: ip, Mask: ipnet.Mask}
}
