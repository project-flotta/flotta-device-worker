package configuration_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-operator/models"
)

const (
	deviceConfigName = "device-config.json"
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
		It("Should use configuration when device-config file is not valid json", func() {
			// given
			err = ioutil.WriteFile(
				fmt.Sprintf("%s/%s", datadir, deviceConfigName),
				[]byte("foo"),
				0640)
			Expect(err).NotTo(HaveOccurred())

			// when
			configManager := configuration.NewConfigurationManager(datadir)

			// then
			Expect(configManager.GetDeviceConfiguration()).To(Equal(getDefaultDeviceconfig()))
		})

	})

	Context("RegisterObserver", func() {

		BeforeEach(func() {
			cfg.Workloads = []*models.Workload{
				{Name: "foo", Specification: ""},
				{Name: "bar", Specification: ""},
			}
			file, err := json.MarshalIndent(cfg, "", " ")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(fmt.Sprintf("%s/%s", datadir, deviceConfigName), file, 0640)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Init is called correctlty", func() {

			// given
			configManager := configuration.NewConfigurationManager(datadir)

			// then
			observerMock := configuration.NewMockObserver(mockCtrl)

			observerMock.EXPECT().Init(gomock.Any()).Do(func(configuration models.DeviceConfigurationMessage) error {
				Expect(configuration.Workloads).To(HaveLen(2))
				Expect(configuration.Workloads[0].Name).To(Equal("foo"))
				Expect(configuration.Workloads[1].Name).To(Equal("bar"))
				return nil
			}).Times(1)

			// when
			configManager.RegisterObserver(observerMock)
		})

		It("If init observer failed, still keep added into observers", func() {

			// given
			configManager := configuration.NewConfigurationManager(datadir)

			// then
			observerMock := configuration.NewMockObserver(mockCtrl)

			observerMock.EXPECT().Init(gomock.Any()).Return(errors.New("Failed"))
			observerMock.EXPECT().Update(gomock.Any()).Do(func(configuration models.DeviceConfigurationMessage) error {
				Expect(configuration.Workloads).To(HaveLen(3))
				return nil
			}).Times(1)

			// when
			configManager.RegisterObserver(observerMock)

			cfg.Workloads = append(cfg.Workloads, &models.Workload{Name: "newOne", Specification: "xx"})
			err := configManager.Update(cfg)

			// then
			Expect(err).To(BeNil())
		})

	})

	Context("Update", func() {

		It("Works as expected", func() {

			// given
			configManager := configuration.NewConfigurationManager(datadir)

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			observerMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
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
			configManager := configuration.NewConfigurationManager(datadir)

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			configManager.RegisterObserver(observerMock)

			failingObserverMock := configuration.NewMockObserver(mockCtrl)
			failingObserverMock.EXPECT().Update(gomock.Any()).Return(fmt.Errorf("failing")).Times(1)
			failingObserverMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
			configManager.RegisterObserver(failingObserverMock)

			thirdObserverMock := configuration.NewMockObserver(mockCtrl)
			thirdObserverMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			thirdObserverMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
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
			err = os.Chmod(datadir, 0444)
			Expect(err).NotTo(HaveOccurred())

			configManager := configuration.NewConfigurationManager(datadir)

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			observerMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
			configManager.RegisterObserver(observerMock)

			failingObserverMock := configuration.NewMockObserver(mockCtrl)
			failingObserverMock.EXPECT().Update(gomock.Any()).Return(fmt.Errorf("failing")).Times(1)
			failingObserverMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
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

			It("Didn't get overwritten if no need", func() {
				// given
				configManager := configuration.NewConfigurationManager(datadir)

				observerMock := configuration.NewMockObserver(mockCtrl)
				observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(0)
				observerMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
				configManager.RegisterObserver(observerMock)

				// when
				err := configManager.Update(cfg)

				// then
				Expect(err).NotTo(HaveOccurred())
			})

			It("Got overwritten if there is a change", func() {

				// given
				cfg.Workloads = []*models.Workload{{
					Data:          &models.DataConfiguration{},
					Name:          "test",
					Specification: "{}",
				}}

				configManager := configuration.NewConfigurationManager(datadir)

				observerMock := configuration.NewMockObserver(mockCtrl)
				observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
				observerMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)
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
			configManager := configuration.NewConfigurationManager(datadir)
			cfg.Version = "1"

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := configManager.GetConfigurationVersion()

			// then
			Expect(res).To(Equal("1"))
		})

		It("Retrieve correctly from config file", func() {
			//given
			cfg.Version = "10"
			file, err := json.MarshalIndent(cfg, "", " ")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(fmt.Sprintf("%s/%s", datadir, deviceConfigName), file, 0640)
			Expect(err).NotTo(HaveOccurred())

			configManager := configuration.NewConfigurationManager(datadir)

			// when
			res := configManager.GetConfigurationVersion()

			// then
			Expect(res).To(Equal("10"))
		})
	})

	Context("GetWorkloads", func() {

		It("When there is no workload", func() {

			//given
			configManager := configuration.NewConfigurationManager(datadir)

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := configManager.GetWorkloads()

			// then
			Expect(res).To(HaveLen(0))
		})

		It("When there are workloads", func() {
			//given
			configManager := configuration.NewConfigurationManager(datadir)

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

			configManager := configuration.NewConfigurationManager(datadir)

			// when
			res := configManager.GetWorkloads()

			// then
			Expect(res).To(HaveLen(1))
			Expect(res).To(Equal(cfg.Workloads))
		})
	})

	Context("Secrets", func() {

		It("When there is no secret", func() {

			//given
			configManager := configuration.NewConfigurationManager(datadir)

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := configManager.GetSecrets()

			// then
			Expect(res).To(HaveLen(0))
		})

		It("When there are secrets", func() {
			//given
			configManager := configuration.NewConfigurationManager(datadir)

			cfg.Secrets = models.SecretList{
				{
					Name: "secret",
					Data: "{}",
				},
			}

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := configManager.GetSecrets()

			// then
			Expect(res).To(HaveLen(1))
			Expect(res).To(Equal(cfg.Secrets))
		})

		It("When retreiving from config file", func() {
			// given
			cfg.Secrets = models.SecretList{
				{
					Name: "secret",
					Data: "{}",
				},
			}

			file, err := json.MarshalIndent(cfg, "", " ")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(fmt.Sprintf("%s/%s", datadir, deviceConfigName), file, 0640)
			Expect(err).NotTo(HaveOccurred())

			configManager := configuration.NewConfigurationManager(datadir)

			// when
			res := configManager.GetSecrets()

			// then
			Expect(res).To(HaveLen(1))
			Expect(res).To(Equal(cfg.Secrets))
		})

		It("secrets equal", func() {
			// given
			secretBefore := models.SecretList{
				&models.Secret{
					Name: "secret1",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret2",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret3",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
			}
			cfg.Secrets = secretBefore

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(1)
			observerMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)

			configManager := configuration.NewConfigurationManager(datadir)
			configManager.RegisterObserver(observerMock)

			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			cfg.Secrets = models.SecretList{
				&models.Secret{
					Name: "secret2",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret3",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret1",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
			}

			// when
			err = configManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(configManager.GetSecrets()).To(Equal(secretBefore))
		})

		It("secrets not equal", func() {
			// given
			cfg.Secrets = models.SecretList{
				&models.Secret{
					Name: "secret1",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret2",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret3",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
			}

			observerMock := configuration.NewMockObserver(mockCtrl)
			observerMock.EXPECT().Update(gomock.Any()).Return(nil).Times(2)
			observerMock.EXPECT().Init(gomock.Any()).Return(nil).Times(1)

			configManager := configuration.NewConfigurationManager(datadir)
			configManager.RegisterObserver(observerMock)
			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			cfg.Secrets = models.SecretList{
				&models.Secret{
					Name: "secret1",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret2",
					Data: `{"key1":"dmFsdWUx","key2":"dmFsdWUy"}`,
				},
				&models.Secret{
					Name: "secret3",
					Data: `{"key2":"dmFsdWUy"}`,
				},
			}

			// when
			err = configManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(configManager.GetSecrets()).To(Equal(cfg.Secrets))
		})
	})

	Context("Deregister", func() {
		It("Delete config file as expected", func() {

			//given
			configManager := configuration.NewConfigurationManager(datadir)

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
			configManager := configuration.NewConfigurationManager(datadir)

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
