package hardware_test

import (
	"net"

	"github.com/jaypipes/ghw"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/project-flotta/flotta-device-worker/internal/hardware"
	"github.com/project-flotta/flotta-operator/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hardware", func() {

	var (
		depMock *util.MockIDependencies
	)

	BeforeEach(func() {
		depMock = &util.MockIDependencies{}
	})
	AfterEach(func() {
		depMock.AssertExpectations(GinkgoT())
	})

	Context("Hardware info test", func() {

		It("Hw full info", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)
			initDependencyMockForImutable(depMock)
			initDependencyMockForMutable(depMock, interfaceMock)

			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.GetHardwareInformation()
			//then
			Expect(err).ToNot(HaveOccurred())
			Expect(hwInfo.CPU).To(Not(BeNil()))
			Expect(hwInfo.Hostname).To(Equal("localhost"))
			Expect(hwInfo.Interfaces).To(Not(BeNil()))
			Expect(hwInfo.SystemVendor).To(Not(BeNil()))
			Expect(hwInfo.HostDevices).To(Not(BeNil()))
		})

		It("Hw immutable info", func() {
			// given

			initDependencyMockForImutable(depMock)
			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo := models.HardwareInfo{}
			err := hw.GetHardwareImmutableInformation(&hwInfo)
			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(hwInfo.CPU).To(Not(BeNil()))
			Expect(hwInfo.Hostname).To(BeEmpty())
			Expect(hwInfo.Interfaces).To(BeNil())
			Expect(hwInfo.SystemVendor).To(Not(BeNil()))
			Expect(hwInfo.HostDevices).To(Not(BeNil()))
		})

		It("Hw mutable info", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)

			initDependencyMockForMutable(depMock, interfaceMock)
			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.CreateHardwareMutableInformation()
			Expect(err).ToNot(HaveOccurred())
			// then
			Expect(hwInfo.CPU).To(BeNil())
			Expect(hwInfo.Hostname).To(Equal("localhost"))
			Expect(hwInfo.Interfaces).To(Not(BeNil()))
			Expect(hwInfo.SystemVendor).To(BeNil())
			Expect(hwInfo.HostDevices).To(BeNil())

		})

		It("Hw not initialized", func() {
			// given
			hw := &hardware.HardwareInfo{}
			// when
			hwInfo, err := hw.CreateHardwareMutableInformation()
			// then
			Expect(err).To(HaveOccurred())
			Expect(hwInfo).To(BeNil())
		})
	})
	Context("Delta Hardware info test", func() {
		It("Hostname changes", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)

			initDependencyMockForImutable(depMock)
			initDependencyMockForMutable(depMock, interfaceMock)

			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.GetHardwareInformation()
			//then
			Expect(err).ToNot(HaveOccurred())

			// given updating Hostname
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Hostname")).To(BeTrue())
			depMock.On("Hostname").Return("localhostNEW", nil)
			// when getting hw info and applying delta to it
			hwInfoNew, err := hw.GetHardwareInformation()
			hwDelta := hw.GetMutableHardwareInfoDelta(*hwInfo, *hwInfoNew)
			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(hwDelta.CPU).To(BeNil())
			Expect(hwDelta.Hostname).To(Equal("localhostNEW"))
			Expect(hwDelta.Interfaces).To(BeNil())
			Expect(hwDelta.SystemVendor).To(BeNil())
			Expect(hwDelta.HostDevices).To(BeNil())
		})
		It("Interface changes", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)

			initDependencyMockForImutable(depMock)
			initDependencyMockForMutable(depMock, interfaceMock)

			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.GetHardwareInformation()
			//then
			Expect(err).ToNot(HaveOccurred())

			// given updating Interface
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Interfaces")).To(BeTrue())
			interfaceMock = NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "100.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, false, 1000)
			depMock.On("Interfaces").Return([]util.Interface{interfaceMock}, nil)
			// when getting hw info and applying delta to it
			hwInfoNew, err := hw.GetHardwareInformation()
			hwDelta := hw.GetMutableHardwareInfoDelta(*hwInfo, *hwInfoNew)
			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(hwDelta.CPU).To(BeNil())
			Expect(hwDelta.Hostname).To(BeEmpty())
			Expect(hwDelta.Interfaces).To(Not(BeNil()))
			Expect(hwDelta.SystemVendor).To(BeNil())
			Expect(hwDelta.HostDevices).To(BeNil())
		})
		It("All mutables change", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)

			initDependencyMockForImutable(depMock)
			initDependencyMockForMutable(depMock, interfaceMock)

			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.GetHardwareInformation()
			//then
			Expect(err).ToNot(HaveOccurred())

			// given updating Hostname and Interfaces
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Hostname")).To(BeTrue())
			depMock.On("Hostname").Return("localhostFinal", nil)
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Interfaces")).To(BeTrue())
			interfaceMock = NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "100.0.0.18/24", "127.0.0.1/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, false, 1000)
			depMock.On("Interfaces").Return([]util.Interface{interfaceMock}, nil)
			// when getting hw info and applying delta to it
			hwInfoNew, err := hw.CreateHardwareMutableInformation()
			Expect(err).ToNot(HaveOccurred())
			hwDelta := hw.GetMutableHardwareInfoDelta(*hwInfo, *hwInfoNew)
			// then
			Expect(hwDelta.CPU).To(BeNil())
			Expect(hwDelta.Hostname).To(Equal("localhostFinal"))
			Expect(hwDelta.Interfaces).To(Not(BeNil()))
			Expect(hwDelta.SystemVendor).To(BeNil())
			Expect(hwDelta.HostDevices).To(BeNil())
		})
		It("No Change", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)

			initDependencyMockForImutable(depMock)
			initDependencyMockForMutable(depMock, interfaceMock)

			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.GetHardwareInformation()
			//then
			Expect(err).ToNot(HaveOccurred())

			// when getting hw information again without any changes and applying delta to it
			hwInfoNew, err := hw.GetHardwareInformation()
			hwDelta := hw.GetMutableHardwareInfoDelta(*hwInfo, *hwInfoNew)
			//then
			Expect(err).ToNot(HaveOccurred())
			Expect(hwDelta.CPU).To(BeNil())
			Expect(hwDelta.Hostname).To(BeEmpty())
			Expect(hwDelta.Interfaces).To(BeNil())
			Expect(hwDelta.SystemVendor).To(BeNil())
			Expect(hwDelta.HostDevices).To(BeNil())
		})
		It("No change if get immutable info only", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)

			initDependencyMockForImutable(depMock)
			initDependencyMockForMutable(depMock, interfaceMock)

			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.GetHardwareInformation()
			//then
			Expect(err).ToNot(HaveOccurred())

			// when getting immutable information and applying delta to it
			hwInfoNew := &models.HardwareInfo{}
			err = hw.GetHardwareImmutableInformation(hwInfoNew)
			Expect(err).ToNot(HaveOccurred())
			hwDelta := hw.GetMutableHardwareInfoDelta(*hwInfo, *hwInfoNew)
			// then
			Expect(hwDelta.CPU).To(BeNil())
			Expect(hwDelta.Hostname).To(BeEmpty())
			Expect(hwDelta.Interfaces).To(BeNil())
			Expect(hwDelta.SystemVendor).To(BeNil())
			Expect(hwDelta.HostDevices).To(BeNil())
		})
		It("Only mutable are checked", func() {
			// given
			interfaceMock := NewFilledInterfaceMock(1500, "eth0", "f8:75:a4:a4:00:fe", net.FlagBroadcast|net.FlagUp, []string{"10.0.0.18/24", "fe80::d832:8def:dd51:3527/128", "de90::d832:8def:dd51:3527/128"}, true, false, true, 1000)

			initDependencyMockForImutable(depMock)
			initDependencyMockForMutable(depMock, interfaceMock)

			hw := &hardware.HardwareInfo{}
			hw.Init(depMock)
			// when
			hwInfo, err := hw.GetHardwareInformation()
			//then
			Expect(err).ToNot(HaveOccurred())

			// given updating immutable information
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Execute")).To(BeTrue())
			Expect(util.DeleteExpectedMethod(&depMock.Mock, "Execute")).To(BeTrue())
			depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "testNEW"}, {"Field" :"Model Name ", "Data": "TestmodelNEW"}]}`, "", 0)
			depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "testNEW"}, {"Field" :"Model Name ", "Data": "TestmodelNEW"}]}`, "", 0)
			// when getting hw info and applying delta to it
			hwInfoNew, err := hw.GetHardwareInformation()
			hwDelta := hw.GetMutableHardwareInfoDelta(*hwInfo, *hwInfoNew)

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(hwDelta.CPU).To(BeNil())
			Expect(hwDelta.Hostname).To(BeEmpty())
			Expect(hwDelta.Interfaces).To(BeNil())
			Expect(hwDelta.SystemVendor).To(BeNil())
			Expect(hwDelta.HostDevices).To(BeNil())
		})
	})

})

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

func initDependencyMockForImutable(depMock *util.MockIDependencies) {
	depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "test"}, {"Field" :"Model Name ", "Data": "Testmodel"}]}`, "", 0)
	depMock.On("Product", mock.AnythingOfType("*option.Option")).Return(&ghw.ProductInfo{SerialNumber: "SerialNumber", Name: "Name", Vendor: "Vendor", Family: "Family"}, nil)
	depMock.On("GetGhwChrootRoot").Return("/host", nil)
}

func initDependencyMockForMutable(depMock *util.MockIDependencies, interfaceMock *util.MockInterface) {
	depMock.On("Execute", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(`{ "Lscpu": [{"Field" :"Architecture ", "Data": "test"}, {"Field" :"Model Name ", "Data": "Testmodel"}]}`, "", 0)
	depMock.On("Hostname").Return("localhost", nil)
	depMock.On("Interfaces").Return([]util.Interface{interfaceMock}, nil)
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
}
