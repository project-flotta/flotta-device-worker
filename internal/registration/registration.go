package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/project-flotta/flotta-device-worker/internal/configuration"
	hardware2 "github.com/project-flotta/flotta-device-worker/internal/hardware"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-operator/models"
	"github.com/redhatinsights/yggdrasil"
	pb "github.com/redhatinsights/yggdrasil/protocol"
)

const (
	retryAfter             = 10
	defaultTargetNamespace = "default"
	targetNamespaceTag     = "namespace"
)

//go:generate mockgen -package=registration -destination=mock_deregistrable.go . Deregistrable
type Deregistrable interface {
	fmt.Stringer
	Deregister() error
}

//go:generate mockgen -package=registration -destination=mock_registration.go . RegistrationWrapper
type RegistrationWrapper interface {
	RegisterDevice()
}

type Registration struct {
	hardware         hardware2.Hardware
	dispatcherClient pb.DispatcherClient
	workloads        *workload.WorkloadManager
	config           *configuration.Manager
	clientCert       *ClientCert
	targetNamepsace  string
	deviceID         string
	deregistrables   []Deregistrable
	RetryAfter       int64
	lock             sync.RWMutex
	registered       bool
}

func NewRegistration(deviceID string, hardware hardware2.Hardware, dispatcherClient DispatcherClient,
	config *configuration.Manager, workloadsManager *workload.WorkloadManager) (*Registration, error) {
	reg := &Registration{
		deviceID:         deviceID,
		hardware:         hardware,
		dispatcherClient: dispatcherClient,
		config:           config,
		RetryAfter:       retryAfter,
		workloads:        workloadsManager,
		lock:             sync.RWMutex{},
	}

	reg.targetNamepsace = reg.getTargetNamespace()
	err := reg.CreateClientCerts()
	if err != nil {
		return nil, err
	}
	return reg, nil
}

func (r *Registration) getTargetNamespace() string {
	filename := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "tags.toml")
	tags, err := readTagsFile(filename)
	if err != nil {
		log.Infof("cannot read tags from yggdrasil config, fp='%s', err: %v", filename, err)
		return defaultTargetNamespace
	}
	val, ok := tags[targetNamespaceTag]
	if !ok {
		return defaultTargetNamespace
	}

	if val == "" {
		return defaultTargetNamespace
	}
	return val
}

func (r *Registration) CreateClientCerts() error {
	data, err := r.dispatcherClient.GetConfig(context.Background(), &pb.Empty{})
	if err != nil {
		return err
	}
	r.clientCert, err = NewClientCert(data.CertFile, data.KeyFile)
	return err
}

func (r *Registration) DeregisterLater(deregistrables ...Deregistrable) {
	r.deregistrables = append(r.deregistrables, deregistrables...)
}

func (r *Registration) RegisterDevice() {
	err := r.registerDeviceOnce()

	if err != nil {
		log.Errorf("cannot register device. DeviceID: %s; err: %v", r.deviceID, err)
	}

	go r.registerDeviceWithRetries(r.RetryAfter)
}

func (r *Registration) registerDeviceWithRetries(interval int64) {
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	for range ticker.C {
		log.Infof("configuration has not been initialized yet. Sending registration request. DeviceID: %s;", r.deviceID)
		err := r.registerDeviceOnce()
		if err != nil {
			log.Errorf("cannot register device. DeviceID: %s; err: %v", r.deviceID, err)
			continue
		}
		ticker.Stop()
	}
}

func (r *Registration) registerDeviceOnce() error {

	enrolled, err := r.enrol()
	if err != nil {
		return err
	}

	if !enrolled {
		return fmt.Errorf("device is not enrolled yet")
	}

	key, csr, err := r.renewCertificate()
	if err != nil {
		return err
	}

	// Call "Send"
	response, err := r.sendRegistrationRequest(csr)
	if err != nil {
		return err
	}

	err = r.parseRegistrationResponse(response, key)
	if err != nil {
		return err
	}

	r.lock.Lock()
	r.registered = true
	r.lock.Unlock()
	return nil
}

func (r *Registration) enrol() (bool, error) {
	hardwareInformation, err := r.hardware.GetHardwareInformation()
	if err != nil {
		return false, err
	}

	enrolInfo := models.EnrolmentInfo{
		Features: &models.EnrolmentInfoFeatures{
			Hardware: hardwareInformation,
		},
		TargetNamespace: &r.targetNamepsace,
	}

	content, err := json.Marshal(enrolInfo)
	if err != nil {
		return false, err
	}

	data := &pb.Data{
		MessageId: uuid.New().String(),
		Content:   content,
		Directive: "enrolment",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Call "Send" to send the enrolment message
	response, err := r.dispatcherClient.Send(ctx, data)
	if err != nil {
		return false, err
	}

	parsedResponse, err := NewYGGDResponse(response.Response)
	if err != nil {
		return false, err
	}

	if parsedResponse.StatusCode == 208 {
		return true, nil
	}
	return false, nil
}

func (r *Registration) renewCertificate() ([]byte, []byte, error) {
	isRegisterCert, err := r.clientCert.IsRegisterCert()
	if err != nil {
		return nil, nil, err
	}
	var key, cert []byte
	if isRegisterCert {
		cert, key, err = r.clientCert.CreateDeviceCerts(r.deviceID)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot create device certs: %v", err)
		}
	} else {
		bufferKey := r.clientCert.certGroup.GetKey()
		if bufferKey == nil {
			return nil, nil, fmt.Errorf("cannot retrieve current key")
		}
		cert, key, err = r.clientCert.Renew(r.deviceID, bufferKey)
		if err != nil {
			return nil, nil, err
		}
	}
	return key, cert, nil
}

func (r *Registration) sendRegistrationRequest(csr []byte) (*pb.Response, error) {
	hardwareInformation, err := r.hardware.GetHardwareInformation()
	if err != nil {
		return nil, err
	}

	registrationInfo := models.RegistrationInfo{
		Hardware:           hardwareInformation,
		CertificateRequest: string(csr),
	}

	content, err := json.Marshal(registrationInfo)
	if err != nil {
		return nil, err
	}

	data := &pb.Data{
		MessageId: uuid.New().String(),
		Content:   content,
		Directive: "registration",
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	response, err := r.dispatcherClient.Send(ctx, data)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (r *Registration) parseRegistrationResponse(response *pb.Response, key []byte) error {
	parsedResponse, err := NewYGGDResponse(response.Response)
	if err != nil {
		return err
	}

	if parsedResponse.StatusCode >= 300 {
		return fmt.Errorf("cannot register to the operator, status_code=%d, body=%s", parsedResponse.StatusCode, parsedResponse.Body)
	}

	var message models.MessageResponse
	err = json.Unmarshal(parsedResponse.Body, &message)
	if err != nil {
		return fmt.Errorf("cannot unmarshal registration response content: %v", err)
	}

	parsedContent, ok := message.Content.(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot parse message content")
	}

	cert, ok := parsedContent["certificate"]
	if !ok {
		return fmt.Errorf("cannot retrieve certificate from parsedResponse")
	}

	parsedCert, ok := cert.(string)
	if !ok {
		return fmt.Errorf("cannot parse certificate from response.Content, content=%+v", message.Content)
	}

	err = r.clientCert.WriteCertificate([]byte(parsedCert), key)
	if err != nil {
		log.Errorf("failed to write certificate: %v,", err)
		return err
	}
	return nil
}

func (r *Registration) IsRegistered() bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.registered
}

func (r *Registration) Deregister() error {
	var errors error
	for _, closer := range r.deregistrables {
		err := closer.Deregister()
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("failed to deregister %s: %v", closer, err))
			log.Errorf("failed to deregister %s. DeviceID: %s; err: %v", closer, r.deviceID, err)
		}
	}

	r.lock.RLock()
	defer r.lock.RUnlock()
	r.registered = false
	return errors
}

type YGGDResponse struct {
	Metadata   map[string]string
	Body       json.RawMessage
	StatusCode int
}

func NewYGGDResponse(response []byte) (*YGGDResponse, error) {
	var parsedResponse YGGDResponse
	err := json.Unmarshal(response, &parsedResponse)
	if err != nil {
		return nil, err
	}
	return &parsedResponse, nil
}
