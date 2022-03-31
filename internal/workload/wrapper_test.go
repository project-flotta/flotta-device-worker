package workload_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/service"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-device-worker/internal/workload/mapping"
	"github.com/project-flotta/flotta-device-worker/internal/workload/network"
	"github.com/project-flotta/flotta-device-worker/internal/workload/podman"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	manifestPath = "/path/to/manifests"
	authFilePath = "/path/to/authfile"
)

var _ = Describe("Workload management", func() {

	var (
		mockCtrl          *gomock.Controller
		wk                *workload.Workload
		newPodman         *podman.MockPodman
		netfilter         *network.MockNetfilter
		mappingRepository *mapping.MockMappingRepository
		serviceManager    *service.MockSystemdManager
		svc               *service.MockService
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		newPodman = podman.NewMockPodman(mockCtrl)
		netfilter = network.NewMockNetfilter(mockCtrl)
		mappingRepository = mapping.NewMockMappingRepository(mockCtrl)
		serviceManager = service.NewMockSystemdManager(mockCtrl)
		svc = service.NewMockService(mockCtrl)

		wk = workload.NewWorkload(newPodman, netfilter, mappingRepository, serviceManager, 15)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Systemd services", func() {

		It("Create service if not exist", func() {

			// given
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}}

			newPodman.EXPECT().Run(manifestPath, authFilePath).Return([]*podman.PodReport{{Id: "id1"}}, nil)
			newPodman.EXPECT().GenerateSystemdService(pod, gomock.Any(), gomock.Any()).Return(svc, nil)

			svc.EXPECT().Add().Return(nil)
			svc.EXPECT().Enable().Return(nil)
			svc.EXPECT().Start().Return(nil)

			serviceManager.EXPECT().Add(svc).Return(nil)

			mappingRepository.EXPECT().Add("pod1", "id1")

			// when
			err := wk.Run(pod, manifestPath, authFilePath)

			// then
			Expect(err).To(BeNil())
		})

		It("Failed to add the service", func() {

			// given
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}}

			mappingRepository.EXPECT().Add("pod1", "id1")

			newPodman.EXPECT().Run(manifestPath, authFilePath).Return([]*podman.PodReport{{Id: "id1"}}, nil)
			newPodman.EXPECT().GenerateSystemdService(pod, gomock.Any(), gomock.Any()).Return(svc, nil)

			svc.EXPECT().Add().Return(fmt.Errorf("Failed to add service"))

			// when
			err := wk.Run(pod, manifestPath, authFilePath)

			// then
			Expect(err).NotTo(BeNil())
		})

		It("Failed to start the service", func() {

			// given
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}}

			mappingRepository.EXPECT().Add("pod1", "id1")

			newPodman.EXPECT().Run(manifestPath, authFilePath).Return([]*podman.PodReport{{Id: "id1"}}, nil)
			newPodman.EXPECT().GenerateSystemdService(pod, gomock.Any(), gomock.Any()).Return(svc, nil)

			svc.EXPECT().Add().Return(nil)
			svc.EXPECT().Enable().Return(fmt.Errorf("Failed to add service"))

			// when
			err := wk.Run(pod, manifestPath, authFilePath)

			// then
			Expect(err).NotTo(BeNil())
		})
	})

})
