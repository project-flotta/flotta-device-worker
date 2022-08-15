package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/url"
	"os"
	"path"
	"reflect"
	"strconv"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/golang/snappy"
	"github.com/project-flotta/flotta-operator/models"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
)

const (
	defaultWaitInterval         = 5 * time.Minute
	defaultRequestDuration      = time.Hour
	defaultRequestRetryInterval = 10 * time.Second
	LastWriteFileName           = "metrics-lastwrite"
	DeviceLabel                 = "edgedeviceid"
	ServerCAFileNamePattern     = "remote-write-ca-*.pem"
)

type RemoteWrite struct {
	lock                 sync.Mutex
	deviceID             string
	Config               *models.MetricsReceiverConfiguration
	tsdb                 API
	LastWrite            time.Time
	lastWriteFile        string
	WaitInterval         time.Duration
	RangeDuration        time.Duration
	RequestRetryInterval time.Duration
	client               WriteClient
	isRunning            bool
	dataDir              string
	currentServerCaFile  string
}

func NewRemoteWrite(dataDir, deviceID string, tsdbInstance API) *RemoteWrite {
	newRemoteWrite := RemoteWrite{
		deviceID:             deviceID,
		tsdb:                 tsdbInstance,
		WaitInterval:         defaultWaitInterval,
		RangeDuration:        defaultRequestDuration,
		RequestRetryInterval: defaultRequestRetryInterval,
		lastWriteFile:        path.Join(dataDir, LastWriteFileName),
		dataDir:              dataDir,
	}

	lastWriteBytes, err := ioutil.ReadFile(newRemoteWrite.lastWriteFile)
	if err == nil {
		if len(lastWriteBytes) != 0 {
			i, err := strconv.ParseInt(string(lastWriteBytes), 10, 64)
			if err == nil {
				newRemoteWrite.LastWrite = time.Unix(i/1000000000, i%1000000000)
			} else {
				log.Errorf("cannot parse metrics last write from file %s. error: %s", newRemoteWrite.lastWriteFile, err.Error())
			}
		}
	} else if !os.IsNotExist(err) {
		log.Errorf("failed reading from file %s. error: %s", newRemoteWrite.lastWriteFile, err.Error())
	}

	return &newRemoteWrite
}

// RemoteRecoverableError
// 'remote.RecoverableError' fields are private and can't be instantiated during testing. Therefore, we use this wrapper
type RemoteRecoverableError struct {
	error
}

func (e RemoteRecoverableError) Error() string {
	if e.error != nil {
		return e.error.Error()
	} else {
		return ""
	}
}

// WriteClient
// an interface used for testing
//go:generate mockgen -package=metrics -destination=write_client_mock.go . WriteClient
type WriteClient interface {
	Write(context.Context, []byte) error
}

type writeClient struct {
	client remote.WriteClient
}

func (w *writeClient) Write(ctx context.Context, data []byte) error {
	err := w.client.Store(ctx, data)
	switch err.(type) {
	case remote.RecoverableError:
		return RemoteRecoverableError{err}
	default:
		return err
	}
}

func (r *RemoteWrite) IsEnabled() bool {
	return r.client != nil
}

func (r *RemoteWrite) IsRunning() bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.isRunning
}

func (r *RemoteWrite) Update(config models.DeviceConfigurationMessage) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	// get new config
	newConfig := (*models.MetricsReceiverConfiguration)(nil)
	if config.Configuration != nil && config.Configuration.Metrics != nil {
		newConfig = config.Configuration.Metrics.Receiver
	}

	// compare with old config
	if r.Config == newConfig || (r.Config != nil && newConfig != nil && reflect.DeepEqual(r.Config, newConfig)) {
		return nil // configuration did not change
	}

	// use new config
	featureDisabled := newConfig == nil || newConfig.URL == ""

	if featureDisabled {
		r.client = nil
		r.currentServerCaFile = ""
		r.removeServerCaFiles()
	} else {
		err := r.applyConfig(newConfig)
		if err != nil {
			return err
		}
	}

	// start goroutine if needed
	if !featureDisabled && !r.isRunning {
		go r.writeRoutine()
		r.isRunning = true
	}

	// set new Config
	r.Config = newConfig

	return nil
}

func (r *RemoteWrite) Init(config models.DeviceConfigurationMessage) error {
	r.removeServerCaFiles()
	return r.Update(config)
}

func (r *RemoteWrite) applyConfig(newConfig *models.MetricsReceiverConfiguration) error {
	// parse and validate new config
	serverURL, err := url.Parse(newConfig.URL)
	if err != nil {
		log.Errorf("metrics remote write configuration is invalid. Can not parse URL %s. Error: %s", newConfig.URL, err.Error())
		return err
	}

	if newConfig.TimeoutSeconds == 0 {
		return fmt.Errorf("metrics remote write configuration is invalid. TimeoutSeconds has to greater than 0")
	}

	if newConfig.RequestNumSamples == 0 {
		return fmt.Errorf("metrics remote write configuration is invalid. RequestNumSamples has to greater than 0")
	}

	// create TLS client config
	clientConfig := config_util.HTTPClientConfig{}

	if newConfig.CaCert != "" {
		// create new file for CA. can't use same file cause client watches for changes in the file we pass to it.
		caFile, err := ioutil.TempFile(r.dataDir, ServerCAFileNamePattern)
		if err != nil {
			log.Errorf("cannot create temp file for metrics remote write server CA at %s", r.dataDir)
			return err
		}

		caFilePath := caFile.Name()

		_, err = caFile.WriteString(newConfig.CaCert)
		if err == nil {
			err = caFile.Sync()
			if err == nil {
				err = caFile.Close()
			}
		}
		if err != nil {
			_ = caFile.Close()
			log.Errorf("cannot write to metrics remote write server CA file %s", caFilePath)
			return err
		}

		clientConfig.TLSConfig = config_util.TLSConfig{
			CAFile: caFilePath,
		}
	}

	// create client
	client, err := remote.NewWriteClient(r.deviceID, &remote.ClientConfig{
		Timeout: model.Duration(newConfig.TimeoutSeconds * int64(time.Second)),
		URL: &config_util.URL{
			URL: serverURL,
		},
		HTTPClientConfig: clientConfig,
	})
	if err != nil {
		_ = os.Remove(clientConfig.TLSConfig.CAFile)
		log.Error("failed creating metrics remote write client", err)
		return err
	}

	if r.currentServerCaFile != "" {
		err = os.Remove(r.currentServerCaFile)
		if err != nil {
			log.Errorf("failed removing file %s with error %s", r.currentServerCaFile, err.Error())
		}
	}

	// update RemoteWrite
	r.client = &writeClient{
		client: client,
	}
	r.currentServerCaFile = clientConfig.TLSConfig.CAFile

	return nil
}

// writeRoutine
// Used as the goroutine function for handling ongoing writes to remote server
func (r *RemoteWrite) writeRoutine() {
	log.Infof("metric remote writer started. %+v", r)

	shouldExit := false
	client := WriteClient(nil)
	requestNumSamples := 0

	getConfig := func() { // we use a function in order to use defer
		r.lock.Lock()
		defer r.lock.Unlock()
		if r.IsEnabled() {
			client = r.client
			requestNumSamples = int(r.Config.RequestNumSamples)
		} else {
			r.isRunning = false
			shouldExit = true
		}
	}

	for {
		getConfig()

		if shouldExit {
			log.Info("metrics remote write stopped")
			return
		}

		r.Write(client, requestNumSamples)
		time.Sleep(r.WaitInterval)
	}
}

// write
// Writes all the metrics. Stops when either no more metrics or failure (after retries)
func (r *RemoteWrite) Write(client WriteClient, requestNumSamples int) {
	hadEmptyRange := false // this is for detecting a 'hole' in the DB and skipping it

	// start writing
	for maxTSDB := r.tsdb.MaxTime(); maxTSDB.Sub(r.LastWrite) > 0; maxTSDB = r.tsdb.MaxTime() {

		// set range start and end
		rangeStart := r.LastWrite.Add(time.Millisecond) // TSDB time is in milliseconds
		minTSDB, err := r.tsdb.MinTime()
		if err != nil {
			log.Error("failed reading TSDB min", err)
			return
		}

		if hadEmptyRange {
			rangeStart = r.skipEmptyRanges(rangeStart)
		} else if rangeStart.Sub(minTSDB) < 0 {
			rangeStart = minTSDB
		}

		rangeEnd := rangeStart.Add(r.RangeDuration)
		if rangeEnd.Sub(maxTSDB) > 0 {
			rangeEnd = maxTSDB
		}

		log.Debugf("going to write metrics range %s(%d)-%s(%d). TSDB min max: %s(%d)-%s(%d)",
			rangeStart.String(), rangeStart.UnixNano(), rangeEnd.String(), rangeEnd.UnixNano(),
			minTSDB.String(), minTSDB.UnixNano(), maxTSDB.String(), maxTSDB.UnixNano(),
		)

		// read range
		series, err := r.tsdb.GetMetricsForTimeRange(rangeStart, rangeEnd, true)
		if err != nil {
			log.Errorf("failed reading metrics for range %d-%d", rangeStart.UnixNano(), rangeEnd.UnixNano())
			return
		}

		// add device label
		for _, s := range series {
			s.Labels[DeviceLabel] = r.deviceID
		}

		// write range
		if len(series) != 0 {
			hadEmptyRange = false
			if r.writeRange(series, client, requestNumSamples) {
				log.Infof("wrote metrics range %s(%d)-%s(%d)", rangeStart.String(), rangeStart.UnixNano(), rangeEnd.String(), rangeEnd.UnixNano())
			} else {
				return // all tries failed. we will retry later
			}
		} else {
			hadEmptyRange = true
			log.Info("metrics range is empty")
		}

		// store last write
		r.LastWrite = rangeEnd
		err = ioutil.WriteFile(r.lastWriteFile, strconv.AppendInt(nil, r.LastWrite.UnixNano(), 10), 0600)
		if err != nil {
			log.Errorf("failed writing to file %s. error: %s", r.lastWriteFile, err.Error())
		}
	}
}

// return value indicates whether or not to continue to next range
// not necessarily that everything was written and successful
func (r *RemoteWrite) writeRange(series []Series, client WriteClient, requestNumSamples int) bool {
	requestSeries := make([]Series, 0, len(series)) // the series slice for current request
	sampleIndex := 0                                // location within current series
	numSamples := 0                                 // how many samples we accumulated for the request

	for i := 0; i < len(series); {
		currentSeries := series[i]
		lenCurrentSeries := len(currentSeries.DataPoints)

		if lenCurrentSeries != 0 {
			if sampleIndex == 0 || len(requestSeries) == 0 { // first time we're using this series or requestSeries was reset
				requestSeries = append(requestSeries, Series{Labels: currentSeries.Labels})
			}

			numMissingSamples := requestNumSamples - numSamples
			remainderOfSeries := lenCurrentSeries - sampleIndex
			numSamplesToUse := numMissingSamples
			if numMissingSamples > remainderOfSeries {
				numSamplesToUse = remainderOfSeries
			}

			samplesToAppend := currentSeries.DataPoints[sampleIndex : sampleIndex+numSamplesToUse]
			numSamples += numSamplesToUse
			sampleIndex += numSamplesToUse

			currReqSeries := &(requestSeries[len(requestSeries)-1])
			currReqSeries.DataPoints = append(currReqSeries.DataPoints, samplesToAppend...)
		}

		if sampleIndex == lenCurrentSeries {
			// advance to next series
			i++
			sampleIndex = 0
		}

		// if request not full and this is not the last series then continue to next series
		if numSamples != requestNumSamples && i < len(series) {
			continue
		}

		if !r.writeRequest(requestSeries, client) {
			return false
		}

		numSamples = 0
		requestSeries = requestSeries[:0]
	}

	return true
}

// return value indicates whether or not to continue to next request
// not necessarily that everything was written and successful
func (r *RemoteWrite) writeRequest(series []Series, client WriteClient) bool {
	timeSeries, lowest, highest := toTimeSeries(series)

	writeRequest := prompb.WriteRequest{
		Timeseries: timeSeries,
	}

	if log.CurrentLevel() >= log.LevelTrace {
		reqBytes, err := json.Marshal(writeRequest)
		if err != nil {
			log.Error("cannot marshal prompb.WriteRequest to json", err)
		}
		log.Tracef("sending write request (lowest %s highest %s): %s", lowest.String(), highest.String(), string(reqBytes))
	}

	reqBytes, err := writeRequest.Marshal()
	if err != nil {
		log.Error("cannot marshal prompb.WriteRequest", err)
		return false
	}

	reqBytes = snappy.Encode(nil, reqBytes)

	const numTries = 3
	for try := 1; try <= numTries; try++ {
		err = client.Write(context.TODO(), reqBytes)
		if err == nil {
			break
		}

		log.Errorf("sending metrics remote write request failed. try No.%d. error: %s", try, err.Error())

		switch err.(type) {
		case RemoteRecoverableError:
			if try == numTries {
				log.Error("aborting metrics remote write request due to too many tries")
				return false
			} else {
				time.Sleep(r.RequestRetryInterval)
			}
		default:
			try = numTries
			// cause loop to break. skip this request cause error not recoverable
			// e.g. already wrote these samples
		}
	}

	return true
}

func (r *RemoteWrite) removeServerCaFiles() {
	fileInfo, err := ioutil.ReadDir(r.dataDir)
	if err != nil {
		log.Errorf("cannot read %s", r.dataDir)
		return
	}

	for _, file := range fileInfo {
		match, _ := path.Match(ServerCAFileNamePattern, file.Name())
		if match {
			_ = os.Remove(path.Join(r.dataDir, file.Name()))
		}
	}
}

// skipEmptyRanges
// Used for skipping ranges that are outside the DB blocks and head
// Given a point in time (less than maxTSDB) find the time of the closest block (or head)
// Return timeVal if timeVal is within a block (or head)
// Otherwise, return the MinTime of the closest block (or head)
func (r *RemoteWrite) skipEmptyRanges(timeVal time.Time) time.Time {
	result := time.Time{}

	for _, block := range r.tsdb.Blocks() {
		if timeVal.Sub(block.MaxTime) > 0 { // timeVal ahead of this block
			continue
		}

		if timeVal.Sub(block.MinTime) < 0 { // timeVal precedes this block
			result = block.MinTime
		} else { // timeVal contained by this block
			result = timeVal
		}

		break
	}

	if result.IsZero() { // no block is ahead or contains timeVal. Check head
		result = r.tsdb.HeadMinTime() // assume timeVal contained by head
		if timeVal.Sub(result) > 0 {  // timeVal precedes head
			result = timeVal
		}
	}

	return result
}

// toTimeSeries
// convert to TimeSeries and returns lowest and highest timestamps
func toTimeSeries(series []Series) (timeSeries []prompb.TimeSeries, lowest time.Time, highest time.Time) {
	mint := int64(math.MaxInt64)
	maxt := int64(0)

	timeSeries = make([]prompb.TimeSeries, len(series))

	for i, s := range series {
		ts := &(timeSeries[i])
		ts.Labels = make([]prompb.Label, 0, len(s.Labels))
		for k, v := range s.Labels {
			ts.Labels = append(ts.Labels, prompb.Label{
				Name:  k,
				Value: v,
			})
		}
		ts.Samples = make([]prompb.Sample, 0, len(s.DataPoints))
		for _, dp := range s.DataPoints {
			ts.Samples = append(ts.Samples, prompb.Sample{
				Value:     dp.Value,
				Timestamp: dp.Time,
			})
			if dp.Time < mint {
				mint = dp.Time
			}
			if dp.Time > maxt {
				maxt = dp.Time
			}
		}
	}

	lowest = fromDbTime(mint)
	highest = fromDbTime(maxt)
	return
}

func (r *RemoteWrite) String() string {
	return "remote write"
}
