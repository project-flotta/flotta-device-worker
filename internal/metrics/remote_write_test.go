package metrics_test

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/golang/snappy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/project-flotta/flotta-operator/models"
	"github.com/prometheus/prometheus/prompb"
)

var _ = Describe("remote write", func() {
	var (
		requestNumSamples = 30000
		deviceID          = "deviceid"
		remoteWrite       *metrics.RemoteWrite
	)

	Context("observer", func() {
		var (
			tsdbInstance metrics.API
		)

		BeforeEach(func() {
			tmpDir, err := ioutil.TempDir("", "metrics")
			Expect(err).ToNot(HaveOccurred())

			tsdbInstance, err = metrics.NewTSDB(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			remoteWrite = metrics.NewRemoteWrite("", deviceID, tsdbInstance)
		})

		AfterEach(func() {
			err := tsdbInstance.Deregister()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Init with feature disabled", func() {
			// given
			configMessage := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{},
			}

			// when
			err := remoteWrite.Init(configMessage)

			// then
			Expect(err).To(BeNil())
			Expect(remoteWrite.IsEnabled()).To(BeFalse())
			Expect(remoteWrite.IsRunning()).To(BeFalse())
		})

		It("Init with feature enabled", func() {
			// given
			configMessage := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						Receiver: &models.MetricsReceiverConfiguration{
							RequestNumSamples: int64(requestNumSamples),
							TimeoutSeconds:    10,
							URL:               "http://something",
						},
					},
				},
			}

			// when
			err := remoteWrite.Init(configMessage)

			// then
			Expect(err).To(BeNil())
			Expect(remoteWrite.IsEnabled()).To(BeTrue())
			Expect(remoteWrite.IsRunning()).To(BeTrue())
		})

		It("Update", func() {
			// given
			configMessage := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						Receiver: &models.MetricsReceiverConfiguration{
							RequestNumSamples: int64(requestNumSamples),
							TimeoutSeconds:    10,
							URL:               "http://something",
						},
					},
				},
			}

			err := remoteWrite.Init(configMessage)
			Expect(err).To(BeNil())

			configMessage2 := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						Receiver: &models.MetricsReceiverConfiguration{
							RequestNumSamples: int64(requestNumSamples + 1),
							TimeoutSeconds:    1,
							URL:               "http://somethingelse",
						},
					},
				},
			}

			// when
			err = remoteWrite.Update(configMessage2)

			// then
			Expect(err).To(BeNil())
			Expect(remoteWrite.IsEnabled()).To(BeTrue())
			Expect(remoteWrite.IsRunning()).To(BeTrue())
			Expect(remoteWrite.Config).To(Equal(configMessage2.Configuration.Metrics.Receiver))
		})

		It("Update - enable feature", func() {
			// given
			configMessage := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{},
			}

			err := remoteWrite.Init(configMessage)
			Expect(err).To(BeNil())

			configMessage2 := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						Receiver: &models.MetricsReceiverConfiguration{
							RequestNumSamples: int64(requestNumSamples),
							TimeoutSeconds:    10,
							URL:               "http://something",
						},
					},
				},
			}

			// when
			err = remoteWrite.Update(configMessage2)

			// then
			Expect(err).To(BeNil())
			Expect(remoteWrite.IsEnabled()).To(BeTrue())
			Expect(remoteWrite.IsRunning()).To(BeTrue())
		})

		It("Update - disable feature", func() {
			// given
			remoteWrite.WaitInterval = 0
			configMessage := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						Receiver: &models.MetricsReceiverConfiguration{
							RequestNumSamples: int64(requestNumSamples),
							TimeoutSeconds:    10,
							URL:               "http://something",
						},
					},
				},
			}

			err := remoteWrite.Init(configMessage)
			Expect(err).To(BeNil())
			Expect(remoteWrite.IsRunning()).To(BeTrue())

			configMessage2 := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						Receiver: &models.MetricsReceiverConfiguration{
							RequestNumSamples: int64(requestNumSamples),
							TimeoutSeconds:    10,
							URL:               "",
						},
					},
				},
			}

			// when
			err = remoteWrite.Update(configMessage2)

			// then
			Expect(err).To(BeNil())
			Expect(remoteWrite.IsEnabled()).To(BeFalse())
			Eventually(remoteWrite.IsRunning, "50ms").Should(BeFalse())
		})
	})

	Context("writer", func() {
		var (
			mockCtrl     *gomock.Controller
			tsdbInstance *metrics.MockAPI
			writeClient  *metrics.MockWriteClient
			minTime      time.Time
		)

		BeforeEach(func() {
			minTime = time.Now()
			os.Remove(metrics.LastWriteFileName)
			mockCtrl = gomock.NewController(GinkgoT())
			tsdbInstance = metrics.NewMockAPI(mockCtrl)
			writeClient = metrics.NewMockWriteClient(mockCtrl)
			remoteWrite = metrics.NewRemoteWrite("", deviceID, tsdbInstance)
		})

		AfterEach(func() {
			defer GinkgoRecover()
			mockCtrl.Finish()
			os.Remove(metrics.LastWriteFileName)
		})

		It("new device", func() {
			//given
			tsdbInstance.EXPECT().MaxTime().Return(time.Time{}).Times(1)
			lastWriteBefore := remoteWrite.LastWrite

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(lastWriteBefore))
		})

		It("first write, DB not empty", func() {
			// given
			midTime := minTime.Add(remoteWrite.RangeDuration)
			maxTime := midTime.Add(time.Millisecond).Add(remoteWrite.RangeDuration)
			series := []metrics.Series{{Labels: map[string]string{}, DataPoints: []metrics.DataPoint{{}}}}
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(2)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(3)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, midTime, true).Return(series, nil).Times(1)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(midTime.Add(time.Millisecond), maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).Times(2)

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(maxTime))
		})

		It("continue from last write", func() {
			// given
			maxTime := minTime.Add(time.Duration(float64(remoteWrite.RangeDuration) * 2.5))
			lastWrite := minTime.Add(remoteWrite.RangeDuration)
			midTime := lastWrite.Add(remoteWrite.RangeDuration).Add(time.Millisecond)
			series := []metrics.Series{{Labels: map[string]string{}, DataPoints: []metrics.DataPoint{{}}}}
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(2)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(3)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(lastWrite.Add(time.Millisecond), midTime, true).Return(series, nil).Times(1)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(midTime.Add(time.Millisecond), maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).Times(2)
			remoteWrite.LastWrite = lastWrite

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(maxTime))
		})

		It("device label", func() {
			// given
			maxTime := minTime
			series := []metrics.Series{{Labels: map[string]string{}, DataPoints: []metrics.DataPoint{{}}}}
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(1)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(2)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, req []byte) {
					obj := decodeRequest(req)
					for _, ts := range obj.Timeseries {
						deviceLabelExists := false
						deviceLabelValue := ""
						for _, l := range ts.Labels {
							if l.Name == metrics.DeviceLabel {
								deviceLabelExists = true
								deviceLabelValue = l.Value
							}
						}
						Expect(deviceLabelExists).To(BeTrue())
						Expect(deviceLabelValue).To(Equal(deviceID))
					}
				}).
				Return(nil).Times(1)

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(maxTime))
		})

		It("number of samples per request - test 1", func() {
			// given
			maxTime := minTime
			series := []metrics.Series{
				{
					Labels:     map[string]string{},
					DataPoints: make([]metrics.DataPoint, requestNumSamples*2),
				},
			}
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(1)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(2)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, req []byte) {
					obj := decodeRequest(req)
					Expect(obj.Timeseries).To(HaveLen(1))
					Expect(obj.Timeseries[0].Samples).To(HaveLen(requestNumSamples))
				}).
				Return(nil).Times(2)

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(maxTime))
		})

		It("number of samples per request - test 2", func() {
			// given
			maxTime := minTime
			series := make([]metrics.Series, 4)
			for i := 0; i < len(series); i++ {
				series[i].Labels = map[string]string{}
			}
			series[0].DataPoints = make([]metrics.DataPoint, requestNumSamples/2)
			series[2].DataPoints = series[0].DataPoints
			series[1].DataPoints = make([]metrics.DataPoint, requestNumSamples*2)
			series[3].DataPoints = series[1].DataPoints
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(1)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(2)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, req []byte) {
					obj := decodeRequest(req)
					numSamples := 0
					for _, ts := range obj.Timeseries {
						numSamples += len(ts.Samples)
					}
					Expect(numSamples).To(Equal(requestNumSamples))
				}).
				Return(nil).Times(5)

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(maxTime))
		})

		It("request send failed - non-recoverable error", func() {
			// given
			maxTime := minTime
			series := []metrics.Series{{Labels: map[string]string{}, DataPoints: []metrics.DataPoint{{}}}}
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(1)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(2)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).Return(errors.New("")).Times(1)

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(maxTime))
		})

		It("request send failed all tries - recoverable error", func() {
			// given
			maxTime := minTime
			series := []metrics.Series{{Labels: map[string]string{}, DataPoints: []metrics.DataPoint{{}}}}
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(1)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(1)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).Return(metrics.RemoteRecoverableError{}).Times(3)
			remoteWrite.RequestRetryInterval = 0
			lastWriteBefore := remoteWrite.LastWrite

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(lastWriteBefore))
		})

		It("request send failed once - recoverable error", func() {
			// given
			maxTime := minTime
			series := []metrics.Series{{Labels: map[string]string{}, DataPoints: []metrics.DataPoint{{}}}}
			tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(1)
			tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(2)
			tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, maxTime, true).Return(series, nil).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).Return(metrics.RemoteRecoverableError{}).Times(1)
			writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			remoteWrite.RequestRetryInterval = 0

			// when
			remoteWrite.Write(writeClient, requestNumSamples)

			// then
			Expect(remoteWrite.LastWrite).To(Equal(maxTime))
		})
	})
})

func decodeRequest(req []byte) prompb.WriteRequest {
	decoded, err := snappy.Decode(nil, req)
	Expect(err).To(BeNil())
	reqObj := prompb.WriteRequest{}
	err = reqObj.Unmarshal(decoded)
	Expect(err).To(BeNil())
	return reqObj
}
