package metrics_test

import (
	"context"
	"errors"
	"os"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/golang/snappy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/prometheus/prometheus/prompb"
)

var _ = Describe("remote write", func() {
	var (
		mockCtrl     *gomock.Controller
		tsdbInstance *metrics.MockAPI
		remoteWrite  *metrics.RemoteWrite
		writeClient  *metrics.MockWriteClient
		deviceID     = "deviceid"
		minTime      time.Time
	)

	BeforeEach(func() {
		minTime = time.Now()
		os.Remove(metrics.LastWriteFileName)
		mockCtrl = gomock.NewController(GinkgoT())
		tsdbInstance = metrics.NewMockAPI(mockCtrl)
		writeClient = metrics.NewMockWriteClient(mockCtrl)
		remoteWrite = metrics.NewRemoteWrite("", deviceID, tsdbInstance)
		remoteWrite.Client = writeClient
	})

	AfterEach(func() {
		defer GinkgoRecover()
		mockCtrl.Finish()
		os.Remove(metrics.LastWriteFileName)
	})

	It("empty URL feature disabled", func() {
		// given
		remoteWrite.Client = nil

		// when
		remoteWrite.Write()

		// then
		Expect(remoteWrite.LastWrite).To(Equal(time.Time{}))
	})

	It("new device", func() {
		//given
		tsdbInstance.EXPECT().MaxTime().Return(time.Time{}).Times(1)
		lastWriteBefore := remoteWrite.LastWrite

		// when
		remoteWrite.Write()

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
		remoteWrite.Write()

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
		remoteWrite.Write()

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
		remoteWrite.Write()

		// then
		Expect(remoteWrite.LastWrite).To(Equal(maxTime))
	})

	It("number of samples per request - test 1", func() {
		// given
		maxTime := minTime
		series := []metrics.Series{
			{
				Labels:     map[string]string{},
				DataPoints: make([]metrics.DataPoint, remoteWrite.RequestNumSamples*2),
			},
		}
		tsdbInstance.EXPECT().MinTime().Return(minTime, nil).Times(1)
		tsdbInstance.EXPECT().MaxTime().Return(maxTime).Times(2)
		tsdbInstance.EXPECT().GetMetricsForTimeRange(minTime, maxTime, true).Return(series, nil).Times(1)
		writeClient.EXPECT().Write(gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, req []byte) {
				obj := decodeRequest(req)
				Expect(obj.Timeseries).To(HaveLen(1))
				Expect(obj.Timeseries[0].Samples).To(HaveLen(remoteWrite.RequestNumSamples))
			}).
			Return(nil).Times(2)

		// when
		remoteWrite.Write()

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
		series[0].DataPoints = make([]metrics.DataPoint, remoteWrite.RequestNumSamples/2)
		series[2].DataPoints = series[0].DataPoints
		series[1].DataPoints = make([]metrics.DataPoint, remoteWrite.RequestNumSamples*2)
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
				Expect(numSamples).To(Equal(remoteWrite.RequestNumSamples))
			}).
			Return(nil).Times(5)

		// when
		remoteWrite.Write()

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
		remoteWrite.Write()

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
		remoteWrite.Write()

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
		remoteWrite.Write()

		// then
		Expect(remoteWrite.LastWrite).To(Equal(maxTime))
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
