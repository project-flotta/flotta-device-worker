package workload_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-operator/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	deviceId = "device-id-123"
	podSpec  = `containers:
    - name: alpine
      image: quay.io/libpod/alpine:latest`
	cmSpec = `kind: ConfigMap
metadata:
  name: mycm
data:
  key1: data
`
)

var _ = Describe("Events", func() {

	var (
		datadir   string
		mockCtrl  *gomock.Controller
		wkManager *workload.WorkloadManager
		wkwMock   *workload.MockWorkloadWrapper
		err       error
	)

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "worloadTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)

		wkwMock.EXPECT().Init().Return(nil).AnyTimes()
		wkManager, err = workload.NewWorkloadManagerWithParamsAndInterval(datadir, wkwMock, 2, deviceId)
		Expect(err).NotTo(HaveOccurred(), "cannot start the Workload Manager")

	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = os.Remove(datadir)
	})

	Context("NonDefaultMonitoringInterval", func() {

		It("Emit events in case of Start failure", func() {

			// given
			workloads := []*models.Workload{}
			workloads = append(workloads, &models.Workload{
				Data:          &models.DataConfiguration{},
				Name:          "stale",
				Specification: "{}",
			})
			wkwMock.EXPECT().Remove("stale").Times(1)

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     workloads,
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{
				{Id: "stale", Name: "stale", Status: "created"},
			}, nil).AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed to start container"))
			wkwMock.EXPECT().PersistConfiguration().AnyTimes()
			wkwMock.EXPECT().Start(gomock.Any()).Return(fmt.Errorf("failed to start container")).AnyTimes()

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())

			// Check no events are generated:
			time.Sleep(5 * time.Second)
			events := wkManager.PopEvents()
			Expect(len(events)).To(BeNumerically(">=", 1))
		})
	})

})

var _ = Describe("Manager", func() {

	var (
		datadir   string
		mockCtrl  *gomock.Controller
		wkManager *workload.WorkloadManager
		wkwMock   *workload.MockWorkloadWrapper
		err       error
	)

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "worloadTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)

		wkwMock.EXPECT().Init().Return(nil).AnyTimes()
		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock, deviceId)
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")

	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = os.Remove(datadir)
	})

	Context("NewWorkloadManagerWithParams", func() {
		// @INFO: Other rules/creation correctly is part of the BeforeEach

		It("Testing invalid datadir", func() {

			// given
			datadir, err = ioutil.TempDir("", "worloadTest")
			err = os.Chmod(datadir, 0444)
			Expect(err).NotTo(HaveOccurred())

			// When
			wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock, deviceId)

			// Then
			Expect(err).To(HaveOccurred())
			Expect(wkManager).To(BeNil())
		})
	})

	Context("Update", func() {
		It("Works as expected", func() {

			// given
			workloads := []*models.Workload{}

			for i := 0; i < 10; i++ {
				wkName := fmt.Sprintf("test%d", i)
				workloads = append(workloads, &models.Workload{
					Data:          &models.DataConfiguration{},
					Name:          wkName,
					Specification: podSpec,
				})
				wkwMock.EXPECT().Remove(wkName).Times(1)
			}

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     workloads,
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Eq("")).AnyTimes()

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			for i := 0; i < 10; i++ {
				wkName := fmt.Sprintf("test%d", i)
				pod := getPodFor(datadir, wkName)
				Expect(pod.Name).To(BeEquivalentTo(wkName))

				additionalDescription := fmt.Sprintf("failing on pod %s", wkName)
				Expect(pod.Spec.Containers).To(HaveLen(1), additionalDescription)
				Expect(pod.Spec.Containers[0].Env).To(HaveLen(1), additionalDescription)
				Expect(pod.Spec.Containers[0].Env[0]).To(BeEquivalentTo(v1.EnvVar{Name: "DEVICE_ID", Value: deviceId}), additionalDescription)
				Expect(pod.Spec.Containers[0].VolumeMounts).To(HaveLen(1), additionalDescription)
				Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(BeEquivalentTo("/export"), additionalDescription)

				Expect(pod.Spec.Volumes).To(HaveLen(1), additionalDescription)
				Expect(pod.Spec.Volumes[0].HostPath).ToNot(BeNil(), additionalDescription)
				Expect(pod.Spec.Volumes[0].Name).To(ContainSubstring("export-"), additionalDescription)
				Expect(pod.Spec.Volumes[0].Name).To(BeEquivalentTo(pod.Spec.Containers[0].VolumeMounts[0].Name), additionalDescription)
			}
		})

		It("Runs workloads with custom auth file", func() {

			// given
			workloads := []*models.Workload{}

			for i := 0; i < 10; i++ {
				wkName := fmt.Sprintf("test%d", i)
				workloads = append(workloads, &models.Workload{
					Name:            wkName,
					Specification:   podSpec,
					ImageRegistries: &models.ImageRegistries{AuthFile: "authFile-" + wkName},
				})
				wkwMock.EXPECT().Remove(wkName).Times(1)
				wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Eq(getAuthPath(datadir, wkName))).Times(1)
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().AnyTimes()

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			for i := 0; i < 10; i++ {
				wkName := fmt.Sprintf("test%d", i)
				pod := getPodFor(datadir, wkName)
				Expect(pod.Name).To(BeEquivalentTo(wkName))

				authFilePath := getAuthPath(datadir, wkName)
				Expect(getAuthPath(datadir, wkName)).To(BeAnExistingFile())
				authFile, _ := ioutil.ReadFile(authFilePath)
				Expect(authFile).To(BeEquivalentTo("authFile-" + wkName))
			}
		})

		It("Runs workloads with configmap", func() {

			// given
			workloads := []*models.Workload{}

			wkName := fmt.Sprintf("test")
			workloads = append(workloads, &models.Workload{
				Name:          wkName,
				Specification: podSpec,
				Configmaps:    models.ConfigmapList{cmSpec},
			})
			wkwMock.EXPECT().Remove(wkName).Times(1)
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().AnyTimes()

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			pod := getPodFor(datadir, wkName)
			Expect(pod.Name).To(BeEquivalentTo(wkName))

			manifestPath := getManifestPath(datadir, wkName)
			Expect(manifestPath).To(BeAnExistingFile())
			manifestFile, _ := ioutil.ReadFile(manifestPath)
			Expect(manifestFile).To(ContainSubstring(cmSpec))
		})

		It("Workload Run failed", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{{
					Data:          &models.DataConfiguration{},
					Name:          "test",
					Specification: "{}",
				}},
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Remove("test").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("cannot run workload")).Times(1)

			// when
			err := wkManager.Update(cfg)
			merr, _ := err.(*multierror.Error)

			// then
			Expect(err).To(HaveOccurred())
			Expect(merr.WrappedErrors()).To(HaveLen(1))
		})

		It("Cannot remove existing workload", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{{
					Data:          &models.DataConfiguration{},
					Name:          "test",
					Specification: "{}",
				}},
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Remove("test").Return(fmt.Errorf("cannot run workload")).Times(1)
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			err := wkManager.Update(cfg)
			merr, _ := err.(*multierror.Error)

			// then
			Expect(err).To(HaveOccurred())
			Expect(merr.WrappedErrors()).To(HaveLen(1))
		})

		It("Some workloads failed", func() {
			// So make sure that all worksloads tried to be executed, even if one
			// failed.

			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{
					{
						Data:          &models.DataConfiguration{},
						Name:          "test",
						Specification: "{}",
					},
					{
						Data:          &models.DataConfiguration{},
						Name:          "testB",
						Specification: "{}",
					},
				},
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Remove("test").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), getManifestPath(datadir, "test"), gomock.Any()).Return(fmt.Errorf("cannot run workload")).Times(1)

			wkwMock.EXPECT().Remove("testB").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), getManifestPath(datadir, "testB"), gomock.Any()).Return(nil).Times(1)

			// when
			err := wkManager.Update(cfg)
			merr, _ := err.(*multierror.Error)

			// then
			Expect(err).To(HaveOccurred())
			Expect(merr.WrappedErrors()).To(HaveLen(1))
		})

		It("Staled workload got deleted if it's not in the config", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{
					{
						Data:          &models.DataConfiguration{},
						Name:          "test",
						Specification: "{}",
					},
					{
						Data:          &models.DataConfiguration{},
						Name:          "testB",
						Specification: "{}",
					},
				},
			}

			currentWorkloads := []api.WorkloadInfo{
				{Id: "stale", Name: "stale", Status: "running"},
			}
			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().Return(currentWorkloads, nil).AnyTimes()

			wkwMock.EXPECT().Remove("test").AnyTimes()
			wkwMock.EXPECT().Remove("testB").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			wkwMock.EXPECT().Remove("stale").Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
		})

		It("Staled workload cannot get deleted", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     []*models.Workload{},
			}

			currentWorkloads := []api.WorkloadInfo{
				{Id: "stale", Name: "stale", Status: "running"},
			}
			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().Return(currentWorkloads, nil).AnyTimes()

			wkwMock.EXPECT().Remove("stale").Return(fmt.Errorf("invalid workload"))

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ListWorkloads", func() {
		It("Return the list correctly", func() {

			// given
			currentWorkloads := []api.WorkloadInfo{
				{Id: "foo", Name: "foo", Status: "running"},
			}
			wkwMock.EXPECT().List().Return(currentWorkloads, nil).AnyTimes()

			// when

			list, err := wkManager.ListWorkloads()

			// then

			Expect(list).To(Equal(currentWorkloads))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Return error correctly", func() {

			// given
			currentWorkloads := []api.WorkloadInfo{}
			wkwMock.EXPECT().List().Return(currentWorkloads, fmt.Errorf("invalid")).AnyTimes()

			// when

			list, err := wkManager.ListWorkloads()

			// then

			Expect(list).To(Equal(currentWorkloads))
			Expect(err).To(HaveOccurred())
		})

	})

	Context("Secrets", func() {
		var (
			cfg models.DeviceConfigurationMessage
		)

		BeforeEach(func() {
			cfg = models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
			}
			wkwMock.EXPECT().List().Return(nil, nil).AnyTimes()
		})

		It("Read secrets failed", func() {
			// given
			wkwMock.EXPECT().ListSecrets().Return(nil, fmt.Errorf("test")).Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("Create secret failed", func() {
			// given
			cfg.Secrets = models.SecretList{
				{
					Name: "secret",
				},
			}
			wkwMock.EXPECT().ListSecrets().Return(nil, nil).Times(1)
			wkwMock.EXPECT().CreateSecret(gomock.Any(), gomock.Any()).Return(errors.New("test")).Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("Update secret failed", func() {
			// given
			cfg.Secrets = models.SecretList{
				{
					Name: "secret",
				},
			}
			wkwMock.EXPECT().ListSecrets().
				Return(map[string]struct{}{
					"secret": {},
				}, nil).Times(1)
			wkwMock.EXPECT().UpdateSecret(gomock.Any(), gomock.Any()).Return(errors.New("test")).Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("Remove secret failed", func() {
			// given
			wkwMock.EXPECT().ListSecrets().
				Return(map[string]struct{}{
					"secret": {},
				}, nil).Times(1)
			wkwMock.EXPECT().RemoveSecret(gomock.Any()).Return(errors.New("test")).Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("No secrets in use", func() {
			// given
			wkwMock.EXPECT().ListSecrets().Return(nil, nil).Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).ToNot(HaveOccurred())
		})

		It("Some secrets fail", func() {
			// given
			cfg.Secrets = models.SecretList{
				{
					Name: "secret1",
				},
				{
					Name: "secret2",
				},
				{
					Name: "secret3",
				},
			}
			wkwMock.EXPECT().ListSecrets().Return(nil, nil).Times(1)
			wkwMock.EXPECT().CreateSecret("secret1", gomock.Any()).Return(nil).Times(1)
			wkwMock.EXPECT().CreateSecret("secret2", gomock.Any()).Return(errors.New("test")).Times(1)
			wkwMock.EXPECT().CreateSecret("secret3", gomock.Any()).Return(nil).Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})

		It("Combination of all CRUD operations", func() {
			// given
			cfg.Secrets = models.SecretList{
				{
					Name: "create1",
				},
				{
					Name: "update1",
				},
				{
					Name: "create2",
				},
				{
					Name: "update2",
				},
			}
			wkwMock.EXPECT().ListSecrets().
				Return(map[string]struct{}{
					"update1": {},
					"remove1": {},
					"update2": {},
					"remove2": {},
				}, nil).Times(1)
			wkwMock.EXPECT().CreateSecret("create1", gomock.Any()).Return(nil).Times(1)
			wkwMock.EXPECT().CreateSecret("create2", gomock.Any()).Return(nil).Times(1)
			wkwMock.EXPECT().UpdateSecret("update1", gomock.Any()).Return(nil).Times(1)
			wkwMock.EXPECT().UpdateSecret("update2", gomock.Any()).Return(nil).Times(1)
			wkwMock.EXPECT().RemoveSecret("remove1").Return(nil).Times(1)
			wkwMock.EXPECT().RemoveSecret("remove2").Return(nil).Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func getPodFor(datadir, wkName string) v1.Pod {
	manifestPath := getManifestPath(datadir, wkName)
	manifest, err := ioutil.ReadFile(manifestPath)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	pod := v1.Pod{}
	err = yaml.Unmarshal(manifest, &pod)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return pod
}

func getManifestPath(datadir string, workloadName string) string {
	return path.Join(getWorkloadsDir(datadir, workloadName), workload.WorkloadFileName)
}

func getAuthPath(datadir string, workloadName string) string {
	return path.Join(getWorkloadsDir(datadir, workloadName), workload.AuthFileName)
}

func getWorkloadsDir(datadir string, workloadName string) string {
	return fmt.Sprintf("%s/workloads/%s", datadir, workloadName)
}
