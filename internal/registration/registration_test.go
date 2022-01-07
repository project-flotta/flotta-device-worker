package registration_test

import (
	"fmt"
	"github.com/jakub-dzon/k4e-device-worker/internal/metrics"
	"io/ioutil"
	osUtil "os"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pb "github.com/redhatinsights/yggdrasil/protocol"

	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-device-worker/internal/datatransfer"
	"github.com/jakub-dzon/k4e-device-worker/internal/hardware"
	"github.com/jakub-dzon/k4e-device-worker/internal/heartbeat"
	"github.com/jakub-dzon/k4e-device-worker/internal/os"
	"github.com/jakub-dzon/k4e-device-worker/internal/registration"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
)

const (
	deviceConfigName = "device-config.json"
)

var _ = Describe("Registration", func() {

	var (
		datadir        string
		mockCtrl       *gomock.Controller
		wkManager      *workload.WorkloadManager
		wkwMock        *workload.MockWorkloadWrapper
		dispatcherMock *registration.MockDispatcherClient
		metricsMock    *metrics.MockAPI
		configManager  *configuration.Manager
		hb             *heartbeat.Heartbeat
		hw             = &hardware.Hardware{}
		monitor        = &datatransfer.Monitor{}
		os             = &os.OS{}
		err            error
	)

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "registrationTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())

		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)
		wkwMock.EXPECT().Init().Return(nil).AnyTimes()

		dispatcherMock = registration.NewMockDispatcherClient(mockCtrl)
		metricsMock = metrics.NewMockAPI(mockCtrl)

		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock, "device-id-123")
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")
		configManager = configuration.NewConfigurationManager(datadir)
		monitor = datatransfer.NewMonitor(wkManager, configManager)

		hb = heartbeat.NewHeartbeatService(dispatcherMock,
			configManager,
			wkManager,
			hw,
			monitor)

	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = osUtil.Remove(datadir)
	})

	RegistrationMatcher := func() gomock.Matcher {
		return regMatcher{}
	}

	Context("RegisterDevice", func() {

		It("Work as expected", func() {

			// given
			reg := registration.NewRegistration(hw, os, dispatcherMock, configManager, hb, wkManager, monitor, metricsMock)

			// then
			dispatcherMock.EXPECT().Send(gomock.Any(), RegistrationMatcher()).Times(1)

			//  when
			reg.RegisterDevice()

			// then
			Expect(reg.IsRegistered()).To(BeTrue())
		})

		It("Try to re-register", func() {

			// given
			reg := registration.NewRegistration(hw, os, dispatcherMock, configManager, hb, wkManager, monitor, metricsMock)
			reg.RetryAfter = 1

			// then
			dispatcherMock.EXPECT().Send(gomock.Any(), gomock.Any()).Return(
				nil, fmt.Errorf("failed")).Times(1)
			dispatcherMock.EXPECT().Send(gomock.Any(), RegistrationMatcher()).Times(1)

			//  when
			reg.RegisterDevice()

			// then
			Eventually(reg.IsRegistered, "5s").Should(BeTrue())
		})

	})

	Context("Deregister", func() {
		var configFile string
		BeforeEach(func() {
			configFile = fmt.Sprintf("%s/%s", datadir, deviceConfigName)
			err = ioutil.WriteFile(
				configFile,
				[]byte("{}"),
				0777)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Works as expected", func() {

			// given
			reg := registration.NewRegistration(hw, os, dispatcherMock, configManager, hb, wkManager, monitor, metricsMock)

			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().RemoveTable().AnyTimes()
			wkwMock.EXPECT().RemoveMappingFile().AnyTimes()
			metricsMock.EXPECT().Deregister()

			// when
			err := reg.Deregister()

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(reg.IsRegistered()).To(BeFalse())

		})

		It("Return error if anything fails", func() {

			// given
			reg := registration.NewRegistration(hw, os, dispatcherMock, configManager, hb, wkManager, monitor, metricsMock)

			wkwMock.EXPECT().List().Return(nil, fmt.Errorf("failed"))
			wkwMock.EXPECT().RemoveTable().AnyTimes()
			wkwMock.EXPECT().RemoveMappingFile().AnyTimes()
			metricsMock.EXPECT().Deregister()

			// when
			err := reg.Deregister()

			// then
			Expect(err).To(HaveOccurred())
			Expect(reg.IsRegistered()).To(BeFalse())
		})

		It("Return error if config file is not present", func() {

			// given
			err := osUtil.Remove(configFile)
			Expect(err).NotTo(HaveOccurred())

			reg := registration.NewRegistration(hw, os, dispatcherMock, configManager, hb, wkManager, monitor, metricsMock)

			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().RemoveTable().AnyTimes()
			wkwMock.EXPECT().RemoveMappingFile().AnyTimes()
			metricsMock.EXPECT().Deregister()

			// when
			err = reg.Deregister()

			// then
			Expect(err).To(HaveOccurred())
			Expect(reg.IsRegistered()).To(BeFalse())
		})

		It("Is able to register after deregister", func() {

			// given
			reg := registration.NewRegistration(hw, os, dispatcherMock, configManager, hb, wkManager, monitor, metricsMock)

			wkwMock.EXPECT().List()
			wkwMock.EXPECT().RemoveTable()
			wkwMock.EXPECT().RemoveMappingFile()
			metricsMock.EXPECT().Deregister()

			err = reg.Deregister()
			Expect(err).NotTo(HaveOccurred())

			reg = registration.NewRegistration(hw, os, dispatcherMock, configManager, hb, wkManager, monitor, metricsMock)

			// then
			dispatcherMock.EXPECT().Send(gomock.Any(), RegistrationMatcher()).Times(1)

			//  when
			reg.RegisterDevice()
			Expect(reg.IsRegistered()).To(BeTrue())
		})

	})

})

// this regMatcher is to validate that registration is send on the protobuf
type regMatcher struct{}

func (regMatcher) Matches(data interface{}) bool {
	res, ok := data.(*pb.Data)
	if !ok {
		return false
	}
	return res.Directive == "registration"
}

func (regMatcher) String() string {
	return "is register action"
}
