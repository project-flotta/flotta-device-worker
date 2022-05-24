package registration_test

import (
	context "context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"time"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/yggdrasil"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	grpc "google.golang.org/grpc"

	"github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/hardware"
	"github.com/project-flotta/flotta-device-worker/internal/registration"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-operator/models"
	"github.com/project-flotta/flotta-operator/pkg/mtls"
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
		hwMock         *hardware.MockHardware
		err            error

		caCert        *certificate
		clientCert    *certificate
		clientCertPem []byte
		regCertPem    []byte
		registerCert  *certificate
		regCertPath   = "/tmp/reg.pem"
		regKeyPath    = "/tmp/reg.key"
		sysconfigPath = filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName)
	)

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "registrationTest")
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(sysconfigPath, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())

		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)
		wkwMock.EXPECT().Init().Return(nil).AnyTimes()

		hwMock = hardware.NewMockHardware(mockCtrl)
		hwMock.EXPECT().GetHardwareInformation().Return(&models.HardwareInfo{
			Hostname: "localhost",
			Interfaces: []*models.Interface{{
				IPV4Addresses: []string{"127.0.0.1", "0.0.0.0"},
			}},
			CPU:          &models.CPU{Architecture: "TestArchi", ModelName: "ModelTest"},
			SystemVendor: &models.SystemVendor{Manufacturer: "ManufacturerTest", ProductName: "ProductTest", SerialNumber: "SerialTest"},
		}, nil).AnyTimes()
		dispatcherMock = registration.NewMockDispatcherClient(mockCtrl)

		dispatcherMock.EXPECT().
			GetConfig(gomock.Any(), gomock.Any()).
			Return(&pb.Config{CertFile: regCertPath, KeyFile: regKeyPath}, nil).
			Times(1)

		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock, deviceID)
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")
		configManager = configuration.NewConfigurationManager(datadir)

		caCert = createCACert()
		clientCert = createClientCert(caCert)
		registerCert = createRegistrationClientCert(caCert)

		clientCertPem = pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: clientCert.certBytes,
		})

		regCertPem, _ = registerCert.DumpToFiles(regCertPath, regKeyPath)
	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = os.Remove(datadir)
		_ = os.Remove(sysconfigPath)
		_ = os.Remove(regCertPath)
		_ = os.Remove(regKeyPath)

	})

	RegistrationMatcher := func() gomock.Matcher {
		return regMatcher{}
	}

	getMessageResponse := func(content models.RegistrationResponse) []byte {
		msgREsponse := models.MessageResponse{
			Directive: "registration",
			MessageID: "foo",
			Content:   content,
		}
		data, err := json.Marshal(msgREsponse)
		Expect(err).NotTo(HaveOccurred())
		return data
	}

	getResponse := func(code int) []byte {
		response := registration.YGGDResponse{StatusCode: code}
		responseData, err := json.Marshal(response)
		Expect(err).NotTo(HaveOccurred())

		return responseData
	}

	getYggdrasilResponse := func(content models.RegistrationResponse) []byte {
		msgResponse := getMessageResponse(content)
		response := registration.YGGDResponse{
			StatusCode: 200,
			Body:       msgResponse,
		}
		responseData, err := json.Marshal(response)
		Expect(err).NotTo(HaveOccurred())
		return responseData
	}

	Context("RegisterDevice", func() {

		Context("Enrol", func() {

			It("Send enrol but not approved yet", func() {
				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).NotTo(HaveOccurred())

				dispatcherMock.EXPECT().
					Send(gomock.Any(), gomock.Any()).
					Return(&pb.Response{Response: getResponse(200)}, nil).
					Do(func(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
						Expect(in.Directive).To(Equal("enrolment"))
						enrolInfo := models.EnrolmentInfo{}
						err := json.Unmarshal(in.Content, &enrolInfo)
						Expect(err).NotTo(HaveOccurred())
						Expect(*enrolInfo.TargetNamespace).To(Equal("default"))
						return &pb.Response{Response: getResponse(200)}, nil
					}).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeFalse())
			})

			It("Send enrol with custom namespace", func() {
				// given
				data := []byte(`namespace="foobar"`)
				err = ioutil.WriteFile(filepath.Join(sysconfigPath, "tags.toml"), data, 0777)
				Expect(err).NotTo(HaveOccurred())

				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).NotTo(HaveOccurred())

				dispatcherMock.EXPECT().
					Send(gomock.Any(), gomock.Any()).
					Return(&pb.Response{Response: getResponse(200)}, nil).
					Do(func(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
						Expect(in.Directive).To(Equal("enrolment"))
						enrolInfo := models.EnrolmentInfo{}
						err := json.Unmarshal(in.Content, &enrolInfo)
						Expect(err).NotTo(HaveOccurred())
						Expect(*enrolInfo.TargetNamespace).To(Equal("foobar"))
						return &pb.Response{Response: getResponse(200)}, nil
					}).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeFalse())
			})

			It("Send enrol with tags without namespace defined", func() {
				// given
				data := []byte(`test="foobar"`)
				err = ioutil.WriteFile(filepath.Join(sysconfigPath, "tags.toml"), data, 0777)
				Expect(err).NotTo(HaveOccurred())

				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).NotTo(HaveOccurred())

				dispatcherMock.EXPECT().
					Send(gomock.Any(), gomock.Any()).
					Return(&pb.Response{Response: getResponse(200)}, nil).
					Do(func(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
						Expect(in.Directive).To(Equal("enrolment"))
						enrolInfo := models.EnrolmentInfo{}
						err := json.Unmarshal(in.Content, &enrolInfo)
						Expect(err).NotTo(HaveOccurred())
						Expect(*enrolInfo.TargetNamespace).To(Equal("default"))
						return &pb.Response{Response: getResponse(200)}, nil
					}).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeFalse())
			})

			It("Send enrol and it's already approved", func() {
				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).NotTo(HaveOccurred())

				// enrol one
				dispatcherMock.EXPECT().
					Send(gomock.Any(), gomock.Any()).
					Return(&pb.Response{Response: getResponse(208)}, nil).
					Do(func(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
						Expect(in.Directive).To(Equal("enrolment"))
						return &pb.Response{Response: getResponse(208)}, nil
					}).
					Times(1)

				msgResponse := getYggdrasilResponse(models.RegistrationResponse{
					Certificate: string(clientCertPem),
				})

				// registration one
				dispatcherMock.EXPECT().
					Send(gomock.Any(), RegistrationMatcher()).
					Return(&pb.Response{Response: msgResponse}, nil).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeTrue())
			})

			It("Not authorized to enrol", func() {
				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).NotTo(HaveOccurred())

				// enrol one
				dispatcherMock.EXPECT().
					Send(gomock.Any(), gomock.Any()).
					Return(&pb.Response{Response: getResponse(401)}, nil).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeFalse())
			})

		})

		Context("Already enrolled", func() {

			BeforeEach(func() {

				// Just enrol before register.
				dispatcherMock.EXPECT().
					Send(gomock.Any(), gomock.Any()).
					Return(&pb.Response{Response: getResponse(208)}, nil).
					Do(func(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
						Expect(in.Directive).To(Equal("enrolment"))
						return &pb.Response{Response: getResponse(208)}, nil
					}).
					Times(1)
			})

			It("Work as expected", func() {

				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).To(BeNil())
				msgResponse := getYggdrasilResponse(models.RegistrationResponse{
					Certificate: string(clientCertPem),
				})

				// then
				dispatcherMock.EXPECT().
					Send(gomock.Any(), RegistrationMatcher()).
					Return(&pb.Response{Response: msgResponse}, nil).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeTrue())

				// cert is overwriten with the client one.
				content, err := ioutil.ReadFile(regCertPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(Equal(clientCertPem))
			})

			It("Server respond  with a 404 on register", func() {

				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).To(BeNil())

				msgResponse := getMessageResponse(models.RegistrationResponse{
					Certificate: string(clientCertPem),
				})

				response := registration.YGGDResponse{
					StatusCode: 404,
					Body:       msgResponse,
				}

				responseData, err := json.Marshal(response)
				Expect(err).NotTo(HaveOccurred())

				// then
				dispatcherMock.EXPECT().
					Send(gomock.Any(), RegistrationMatcher()).
					Return(&pb.Response{Response: responseData}, nil).
					Times(1)

					//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeFalse())
				// make sure that it's not overwriten
				content, err := ioutil.ReadFile(regCertPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(Equal(regCertPem))
			})

			It("Server respond  without certificate", func() {

				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).To(BeNil())
				msgResponse := getYggdrasilResponse(models.RegistrationResponse{
					Certificate: "",
				})

				// then
				dispatcherMock.EXPECT().
					Send(gomock.Any(), RegistrationMatcher()).
					Return(&pb.Response{Response: msgResponse}, nil).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeFalse())

				// make sure that it's not overwriten
				content, err := ioutil.ReadFile(regCertPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(Equal(regCertPem))
			})

			It("Server respond with invalid certificate", func() {

				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).To(BeNil())
				msgResponse := getYggdrasilResponse(models.RegistrationResponse{
					Certificate: "XXXX",
				})

				// then
				dispatcherMock.EXPECT().
					Send(gomock.Any(), RegistrationMatcher()).
					Return(&pb.Response{Response: msgResponse}, nil).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeFalse())

				// make sure that it's not overwriten
				content, err := ioutil.ReadFile(regCertPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(Equal(regCertPem))
			})

			It("Renew certificate on client certificate", func() {

				// given
				// Just dump client certificates to there to renew then.
				clientCert.DumpToFiles(regCertPath, regKeyPath)

				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).To(BeNil())
				msgResponse := getYggdrasilResponse(models.RegistrationResponse{
					Certificate: string(clientCertPem),
				})

				// then
				dispatcherMock.EXPECT().
					Send(gomock.Any(), RegistrationMatcher()).
					Do(func(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) {
						var request models.RegistrationInfo
						err := json.Unmarshal(in.Content, &request)
						Expect(err).NotTo(HaveOccurred())

						p, next := pem.Decode([]byte(request.CertificateRequest))
						Expect(next).To(HaveLen(0))
						csr, err := x509.ParseCertificateRequest(p.Bytes)
						Expect(err).NotTo(HaveOccurred())
						Expect(csr.Subject.CommonName).To(Equal("device-id-123"))
					}).
					Return(&pb.Response{Response: msgResponse}, nil).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Expect(reg.IsRegistered()).To(BeTrue())

				content, err := ioutil.ReadFile(regCertPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(content).To(Equal(clientCertPem))
			})

			It("Try to re-register", func() {
				// given
				reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
				Expect(err).NotTo(HaveOccurred())

				msgResponse := getYggdrasilResponse(models.RegistrationResponse{
					Certificate: string(clientCertPem),
				})

				reg.RetryAfter = 1

				// then
				dispatcherMock.EXPECT().Send(gomock.Any(), gomock.Any()).Return(
					nil, fmt.Errorf("failed")).Times(1)

				dispatcherMock.EXPECT().
					Send(gomock.Any(), gomock.Any()).
					Return(&pb.Response{Response: getResponse(208)}, nil).
					Do(func(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
						Expect(in.Directive).To(Equal("enrolment"))
						return &pb.Response{Response: getResponse(208)}, nil
					}).
					Times(1)
				dispatcherMock.EXPECT().
					Send(gomock.Any(), RegistrationMatcher()).
					Return(&pb.Response{Response: msgResponse}, nil).
					Times(1)

				//  when
				reg.RegisterDevice()

				// then
				Eventually(reg.IsRegistered, "5s").Should(BeTrue())
			})

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
			reg, _ := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)

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
			reg, _ := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)

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
			dispatcherMock.EXPECT().
				GetConfig(gomock.Any(), gomock.Any()).
				Return(&pb.Config{CertFile: regCertPath, KeyFile: regKeyPath}, nil).
				Times(1)

			reg, err := registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
			Expect(err).NotTo(HaveOccurred())
			msgResponse := getYggdrasilResponse(models.RegistrationResponse{
				Certificate: string(clientCertPem),
			})

			dispatcherMock.EXPECT().
				Send(gomock.Any(), gomock.Any()).
				Return(&pb.Response{Response: getResponse(208)}, nil).
				Times(1)

			dispatcherMock.EXPECT().
				Send(gomock.Any(), gomock.Any()).
				Return(&pb.Response{Response: msgResponse}, nil).
				Times(1)

			deregistrable := registration.NewMockDeregistrable(mockCtrl)
			deregistrable.EXPECT().Deregister()
			reg.DeregisterLater(deregistrable)

			err = reg.Deregister()
			Expect(err).NotTo(HaveOccurred())

			reg, err = registration.NewRegistration(deviceID, hwMock, dispatcherMock, configManager, wkManager)
			Expect(err).NotTo(HaveOccurred())

			//  when
			reg.RegisterDevice()
			Expect(reg.IsRegistered()).To(BeTrue())

			// then
			content, err := ioutil.ReadFile(regCertPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal(clientCertPem))
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

type certificate struct {
	key        crypto.Signer
	cert       *x509.Certificate
	signedCert *x509.Certificate
	certBytes  []byte
}

func (c *certificate) DumpToFiles(certPath, keyPath string) ([]byte, []byte) {
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.certBytes,
	})

	err := ioutil.WriteFile(certPath, certPEM, 0777)
	Expect(err).NotTo(HaveOccurred())

	keyBytes, err := x509.MarshalECPrivateKey(c.key.(*ecdsa.PrivateKey))
	Expect(err).NotTo(HaveOccurred())

	certKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  mtls.ECPrivateKeyBlockType,
		Bytes: keyBytes,
	})
	err = ioutil.WriteFile(keyPath, certKeyPEM, 0777)
	Expect(err).NotTo(HaveOccurred())
	return certPEM, certKeyPEM
}

func createRegistrationClientCert(ca *certificate) *certificate {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Flotta-operator"},
			CommonName:   mtls.CertRegisterCN,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	return createGivenClientCert(cert, ca)
}

func createClientCert(ca *certificate) *certificate {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Flotta-operator"},
			CommonName:   "device-UUID",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	return createGivenClientCert(cert, ca)
}

func createGivenClientCert(cert *x509.Certificate, ca *certificate) *certificate {
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ExpectWithOffset(1, err).To(BeNil(), "Fail on key generation")

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca.cert, &certKey.PublicKey, ca.key)
	ExpectWithOffset(1, err).To(BeNil(), "Fail on sign generation")

	signedCert, err := x509.ParseCertificate(certBytes)
	ExpectWithOffset(1, err).To(BeNil(), "Fail on parsing certificate")

	err = signedCert.CheckSignatureFrom(ca.signedCert)
	ExpectWithOffset(1, err).To(BeNil(), "Fail on check signature")

	return &certificate{certKey, cert, signedCert, certBytes}
}

func createCACert() *certificate {
	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ExpectWithOffset(1, err).To(BeNil(), "Fail on key generation")
	return createCACertUsingKey(caPrivKey)
}

func createCACertUsingKey(key crypto.Signer) *certificate {

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Flotta-operator"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, key.Public(), key)
	ExpectWithOffset(1, err).To(BeNil(), "Fail on sign generation")

	signedCert, err := x509.ParseCertificate(caBytes)
	ExpectWithOffset(1, err).To(BeNil(), "Fail on parsing certificate")

	return &certificate{key, ca, signedCert, caBytes}
}
