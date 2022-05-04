package heartbeat_test

import (
	"context"
	"fmt"
	"net"

	"github.com/jaypipes/ghw"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-device-worker/internal/ansible"
	os2 "github.com/project-flotta/flotta-device-worker/internal/os"
	"github.com/project-flotta/flotta-device-worker/internal/registration"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

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
		hw             = &hardware.Hardware{}
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
			hw,
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
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hw, monitor, deviceOs)

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))
			Expect(heartbeatInfo.Workloads).To(BeEmpty())
		})

		It("Report workload correctly", func() {
			//given
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hw, monitor, deviceOs)

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
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hw, monitor, deviceOs)

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
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, ansibleManager, hw, monitor, deviceOs)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{}, fmt.Errorf("invalid list")).AnyTimes()

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))
			Expect(heartbeatInfo.Workloads).To(HaveLen(0))
		})

		It("Report workload delta enable", func() {
			//given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)
			depMock := &util.MockIDependencies{}
			depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "test"}, {"Field" :"Model Name ", "Data": "Testmodel"}]}`, "", 0)
			depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "test"}, {"Field" :"Model Name ", "Data": "Testmodel"}]}`, "", 0)
			depMock.On("Hostname").Return("localhost", nil)
			depMock.On("Interfaces").Return([]util.Interface{interfaceMock}, nil)
			depMock.On("Product", mock.AnythingOfType("*option.Option")).Return(&ghw.ProductInfo{SerialNumber: "SerialNumber", Name: "Name", Vendor: "Vendor", Family: "Family"}, nil)
			depMock.On("GetGhwChrootRoot").Return("/host", nil)
			depMock.On("ReadFile", "/sys/class/net/eth0/carrier").Return([]byte("1\n"), nil)
			depMock.On("ReadFile", "/sys/class/net/eth0/device/device").Return([]byte("my-device"), nil)
			depMock.On("ReadFile", "/sys/class/net/eth0/device/vendor").Return([]byte("my-vendor"), nil)
			depMock.On("LinkByName", "eth0").Return(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "eth0"}}, nil)
			depMock.On("RouteList", mock.Anything, mock.Anything).Return([]netlink.Route{
				{
					Dst:      &net.IPNet{IP: net.ParseIP("de90::"), Mask: net.CIDRMask(64, 128)},
					Protocol: unix.RTPROT_RA,
				},
			}, nil)
			hwMock := &hardware.Hardware{
				Dependencies: depMock,
			}
			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Scope = heartbeat.SCOPE_DELTA
			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Include = true
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
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Hostname")).To(BeTrue())
			depMock.On("Hostname").Return("localhostNEW", nil)

			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal("localhostNEW"))
			Expect(heartbeatInfo.Hardware.Interfaces).To(BeNil())
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

			// Hardware checks delta time interface	 change
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Interfaces")).To(BeTrue())
			interfaceMock = NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "100.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, false, 1000)
			depMock.On("Interfaces").Return([]util.Interface{interfaceMock}, nil)

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
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Hostname")).To(BeTrue())
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Interfaces")).To(BeTrue())

			depMock.On("Hostname").Return("localhostFINAL", nil)
			interfaceMock = NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "127.0.0.1/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, false, 1000)
			depMock.On("Interfaces").Return([]util.Interface{interfaceMock}, nil)
			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal("localhostFINAL"))
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

		})

		It("Report workload delta disable", func() {
			//given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)
			depMock := &util.MockIDependencies{}
			depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "test"}, {"Field" :"Model Name ", "Data": "Testmodel"}]}`, "", 0)
			depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "test"}, {"Field" :"Model Name ", "Data": "Testmodel"}]}`, "", 0)
			depMock.On("Hostname").Return("localhost", nil)
			depMock.On("Interfaces").Return([]util.Interface{interfaceMock}, nil)
			depMock.On("Product", mock.AnythingOfType("*option.Option")).Return(&ghw.ProductInfo{SerialNumber: "SerialNumber", Name: "Name", Vendor: "Vendor", Family: "Family"}, nil)
			depMock.On("GetGhwChrootRoot").Return("/host", nil)
			depMock.On("ReadFile", "/sys/class/net/eth0/carrier").Return([]byte("1\n"), nil)
			depMock.On("ReadFile", "/sys/class/net/eth0/device/device").Return([]byte("my-device"), nil)
			depMock.On("ReadFile", "/sys/class/net/eth0/device/vendor").Return([]byte("my-vendor"), nil)
			depMock.On("LinkByName", "eth0").Return(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "eth0"}}, nil)
			depMock.On("RouteList", mock.Anything, mock.Anything).Return([]netlink.Route{
				{
					Dst:      &net.IPNet{IP: net.ParseIP("de90::"), Mask: net.CIDRMask(64, 128)},
					Protocol: unix.RTPROT_RA,
				},
			}, nil)
			hwMock := &hardware.Hardware{
				Dependencies: depMock,
			}
			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Scope = heartbeat.SCOPE_FULL
			configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Include = true
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

			// Hardware checks first time
			Expect(heartbeatInfo.Hardware.CPU).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal("localhost"))
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(Not(BeNil()))

			// Hardware checks second time: only mutable info
			heartbeatInfo = hbData.RetrieveInfo()
			Expect(heartbeatInfo.Hardware.CPU).To(BeNil())
			Expect(heartbeatInfo.Hardware.Hostname).To(Equal("localhost"))
			Expect(heartbeatInfo.Hardware.Interfaces).To(Not(BeNil()))
			Expect(heartbeatInfo.Hardware.SystemVendor).To(BeNil())

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
