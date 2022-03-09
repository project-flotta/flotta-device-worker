package registration_test

import (
	"fmt"
	"io/ioutil"
	osUtil "os"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pb "github.com/redhatinsights/yggdrasil/protocol"

	"github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/hardware"
	"github.com/project-flotta/flotta-device-worker/internal/registration"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
)

const (
	deviceConfigName = "device-config.json"
	deviceID         = "device-id-123"
)

var _ = Describe("Registration", func() {

	var (
		datadir        string
		mockCtrl       *gomock.Controller
		wkManager      *workload.WorkloadManager
		wkwMock        *workload.MockWorkloadWrapper
		dispatcherMock *registration.MockDispatcherClient
		configManager  *configuration.Manager
		hw             = &hardware.Hardware{}
		err            error
	)

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "registrationTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())

		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)
		wkwMock.EXPECT().Init().Return(nil).AnyTimes()

		dispatcherMock = registration.NewMockDispatcherClient(mockCtrl)

		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock, deviceID)
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")
		configManager = configuration.NewConfigurationManager(datadir)
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
			reg := registration.NewRegistration(deviceID, hw, dispatcherMock, configManager, wkManager)

			// then
			dispatcherMock.EXPECT().Send(gomock.Any(), RegistrationMatcher()).Times(1)

			//  when
			reg.RegisterDevice()

			// then
			Expect(reg.IsRegistered()).To(BeTrue())
		})

		It("Try to re-register", func() {
			// given
			reg := registration.NewRegistration(deviceID, hw, dispatcherMock, configManager, wkManager)
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
			reg := registration.NewRegistration(deviceID, hw, dispatcherMock, configManager, wkManager)

			deregistrable1 := registration.NewMockDeregistrable(mockCtrl)
			deregistrable2 := registration.NewMockDeregistrable(mockCtrl)
			deregistrable2.EXPECT().Deregister().After(
				deregistrable1.EXPECT().Deregister(),
			)
			deregistrable1.EXPECT().String().AnyTimes()
			deregistrable2.EXPECT().String().AnyTimes()
			reg.DeregisterLater(deregistrable1, deregistrable2)

			// when
			err := reg.Deregister()

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(reg.IsRegistered()).To(BeFalse())
		})

		It("Return error if anything fails", func() {
			// given
			reg := registration.NewRegistration(deviceID, hw, dispatcherMock, configManager, wkManager)

			deregistrable := registration.NewMockDeregistrable(mockCtrl)
			deregistrable.EXPECT().Deregister().Return(fmt.Errorf("boom"))
			deregistrable.EXPECT().String().AnyTimes()
			reg.DeregisterLater(deregistrable)

			// when
			err := reg.Deregister()

			// then
			Expect(err).To(HaveOccurred())
			Expect(reg.IsRegistered()).To(BeFalse())
		})

		It("Is able to register after deregister", func() {
			// given
			reg := registration.NewRegistration(deviceID, hw, dispatcherMock, configManager, wkManager)

			deregistrable := registration.NewMockDeregistrable(mockCtrl)
			deregistrable.EXPECT().Deregister()
			reg.DeregisterLater(deregistrable)

			err = reg.Deregister()
			Expect(err).NotTo(HaveOccurred())

			reg = registration.NewRegistration(deviceID, hw, dispatcherMock, configManager, wkManager)

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
