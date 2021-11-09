package configuration_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/yaml"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-operator/models"
)

const (
	deviceConfigName = "device-config.json"
	commonDeviceId   = "device-id-123"
)

var _ = Describe("Configuration", func() {

	var (
		datadir             string
		err                 error
		mockCtrl            *gomock.Controller
		cfg                 models.DeviceConfigurationMessage
		deviceConfiguration models.DeviceConfiguration
	)

	deviceConfigExists := func() {
		_, err := os.Stat(fmt.Sprintf("%s/%s", datadir, deviceConfigName))
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
	}

	getDefaultDeviceconfig := func() models.DeviceConfiguration {
		return models.DeviceConfiguration{
			Heartbeat: &models.HeartbeatConfiguration{
				HardwareProfile: &models.HardwareProfileConfiguration{
					Include: false,
					Scope:   "",
				},
				PeriodSeconds: 60,
			},
			Storage: nil,
		}
	}

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "worloadTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())

		deviceConfiguration = models.DeviceConfiguration{
			Heartbeat: &models.HeartbeatConfiguration{
				PeriodSeconds: 1,
			}}

		cfg = models.DeviceConfigurationMessage{
			Configuration:               &deviceConfiguration,
			DeviceID:                    "",
			Version:                     "",
			Workloads:                   []*models.Workload{},
			WorkloadsMonitoringInterval: 0,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = os.Remove(datadir)
	})

	Context("NewConfigurationManager", func() {
		It("should use configuration when device-config file is not valid json", func() {
			// given
			err = ioutil.WriteFile(
				fmt.Sprintf("%s/%s", datadir, deviceConfigName),
				[]byte("foo"),
				0640)
			Expect(err).NotTo(HaveOccurred())

			// when
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			// then
			Expect(configManager.GetDeviceConfiguration()).To(Equal(getDefaultDeviceconfig()))
			Expect(configManager.GetDeviceConfigMapPath()).To(BeAnExistingFile())
			Expect(configManager.GetDeviceId()).To(BeEquivalentTo(commonDeviceId))
			deviceConfigMapExists(configManager.GetDeviceConfigMapPath(), commonDeviceId)
		})

		It("should fail when cannot write device config map", func() {
			// given
			err = os.Chmod(datadir, 0444)
			Expect(err).NotTo(HaveOccurred())

			// when
			_, err = configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			// then
			Expect(err).To(HaveOccurred())
		})

	})

	Context("Update", func() {

		It("works as expected", func() {

			// given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			configManager.RegisterObserver(observerMock)

			// when
			err := configManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(configManager.GetDeviceConfiguration()).To(Equal(deviceConfiguration))
			deviceConfigExists()
		})

		It("One Observer failed", func() {
			// Observer can fail, BUT device config should be created.

			// given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			configManager.RegisterObserver(observerMock)

			failingObserverMock := configuration.NewMockObserver(mockCtrl)
			failingObserverMock.EXPECT().Update(gomock.Any()).Return(fmt.Errorf("failing")).Times(1)
			configManager.RegisterObserver(failingObserverMock)

			thirdObserverMock := configuration.NewMockObserver(mockCtrl)
			thirdObserverMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			configManager.RegisterObserver(thirdObserverMock)

			// when
			err := configManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
			Expect(configManager.GetDeviceConfiguration()).To(Equal(deviceConfiguration))
			deviceConfigExists()
		})

		It("Cannot write device config", func() {

			// given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)
			err = os.Chmod(datadir, 0444)

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			configManager.RegisterObserver(observerMock)

			failingObserverMock := configuration.NewMockObserver(mockCtrl)
			failingObserverMock.EXPECT().Update(gomock.Any()).Return(fmt.Errorf("failing")).Times(1)
			configManager.RegisterObserver(failingObserverMock)

			// when
			err := configManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())

			// Just because cannot update the config
			Expect(configManager.GetDeviceConfiguration()).To(Equal(getDefaultDeviceconfig()))
		})

		Context("With initial configuration", func() {
			var cfg models.DeviceConfigurationMessage

			BeforeEach(func() {
				file, err := json.MarshalIndent(cfg, "", " ")
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(fmt.Sprintf("%s/%s", datadir, deviceConfigName), file, 0640)
				Expect(err).NotTo(HaveOccurred())
			})

			It("didn't get overwritten if no need", func() {
				// given
				configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

				observerMock := configuration.NewMockObserver(mockCtrl)
				observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(0)
				configManager.RegisterObserver(observerMock)

				// when
				err := configManager.Update(cfg)

				// then
				Expect(err).NotTo(HaveOccurred())
			})

			It("got overwritten if there is a change", func() {

				// given
				cfg.Workloads = []*models.Workload{{
					Data:          &models.DataConfiguration{},
					Name:          "test",
					Specification: "{}",
				}}

				configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

				observerMock := configuration.NewMockObserver(mockCtrl)
				observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
				configManager.RegisterObserver(observerMock)

				// when
				err := configManager.Update(cfg)

				// then
				Expect(err).NotTo(HaveOccurred())
			})

		})
	})

	Context("GetConfigurationVersion", func() {

		It("Works as expected", func() {
			//given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)
			cfg.Version = "1"

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := configManager.GetConfigurationVersion()

			// then
			Expect(res).To(Equal("1"))
		})

		It("retrieve correctly from config file", func() {
			//given
			cfg.Version = "10"
			file, err := json.MarshalIndent(cfg, "", " ")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(fmt.Sprintf("%s/%s", datadir, deviceConfigName), file, 0640)
			Expect(err).NotTo(HaveOccurred())

			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			// when
			res := configManager.GetConfigurationVersion()

			// then
			Expect(res).To(Equal("10"))
		})
	})

	Context("GetWorkloads", func() {

		It("When there is no workload", func() {

			//given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := configManager.GetWorkloads()

			// then
			Expect(res).To(HaveLen(0))
		})

		It("When there are workloads", func() {
			//given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			cfg.Workloads = []*models.Workload{{
				Data:          &models.DataConfiguration{},
				Name:          "test",
				Specification: "{}",
			}}
			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := configManager.GetWorkloads()

			// then
			Expect(res).To(HaveLen(1))
			Expect(res).To(Equal(cfg.Workloads))
		})

		It("When retreiving from config file", func() {
			// given

			cfg.Workloads = []*models.Workload{{
				Data:          &models.DataConfiguration{},
				Name:          "test",
				Specification: "{}",
			}}

			file, err := json.MarshalIndent(cfg, "", " ")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(fmt.Sprintf("%s/%s", datadir, deviceConfigName), file, 0640)
			Expect(err).NotTo(HaveOccurred())

			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			// when
			res := configManager.GetWorkloads()

			// then
			Expect(res).To(HaveLen(1))
			Expect(res).To(Equal(cfg.Workloads))
		})
	})

	Context("Deregister", func() {
		It("Delete config file as expected", func() {

			//given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			deviceConfigExists()

			// when
			err = configManager.Deregister()

			// then
			Expect(err).NotTo(HaveOccurred())
		})

		It("Raise an error if cannot be deleted", func() {

			//given
			configManager, _ := configuration.NewConfigurationManagerWithDeviceId(datadir, commonDeviceId)

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			deviceConfigExists()

			err = os.Chmod(datadir, 0444)
			Expect(err).NotTo(HaveOccurred())

			// when
			err = configManager.Deregister()

			// then
			Expect(err).To(HaveOccurred())
		})

	})
})

func deviceConfigMapExists(cmPath string, deviceId string) {
	data, err := ioutil.ReadFile(cmPath)
	cm := v1.ConfigMap{}
	err = yaml.Unmarshal(data, &cm)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, cm.Data["DEVICE_ID"]).To(BeEquivalentTo(deviceId))
	ExpectWithOffset(1, cm.Name).To(BeEquivalentTo(configuration.DeviceConfigMapName))
}
