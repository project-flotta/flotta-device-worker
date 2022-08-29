package registration

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"os"

	"encoding/pem"

	"github.com/project-flotta/flotta-operator/pkg/mtls"
	log "github.com/sirupsen/logrus"
)

type ClientCert struct {
	keyPath   string
	certPath  string
	certGroup *mtls.CertificateGroup
}

func NewClientCert(certPath string, keyPath string) (*ClientCert, error) {
	res := &ClientCert{
		certPath: certPath,
		keyPath:  keyPath,
	}
	err := res.init()
	return res, err
}

func (c *ClientCert) init() error {
	certPem, err := os.ReadFile(c.certPath)
	if err != nil {
		return err
	}

	certKey, err := os.ReadFile(c.keyPath)
	if err != nil {
		return err
	}
	c.certGroup = &mtls.CertificateGroup{
		CertPEM:    bytes.NewBuffer(certPem),
		PrivKeyPEM: bytes.NewBuffer(certKey),
	}
	return c.certGroup.ImportFromPem()
}

func (c *ClientCert) IsRegisterCert() (bool, error) {
	if c.certGroup == nil {
		return false, errors.New("certificates are not imported")
	}

	cert := c.certGroup.GetCert()
	if cert == nil {
		return false, errors.New("no valid cert")
	}

	return cert.Subject.CommonName == mtls.CertRegisterCN, nil
}

// Renew renews the current certificates and issue a new CSR with the given
// key. returns the a new CSR PEM, a key in PEM format and an error if happens.
func (c *ClientCert) Renew(deviceID string, key crypto.PrivateKey) ([]byte, []byte, error) {
	var csrTemplate = x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: deviceID,
			// Operator will add metadata on this subject, like namespace
		},
	}

	csrCertificate, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, key)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrCertificate,
	})

	keyPEM, err := c.certGroup.MarshalKeyToPem(key)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot encode new key: %v", err)
	}

	return certPEM, keyPEM.Bytes(), nil
}

// CreateDeviceCerts creates a new CSR and key  for the given deviceID. It
// returns the CSR in PEM format, Key in PEM format and and error if it happens
func (c *ClientCert) CreateDeviceCerts(deviceID string) ([]byte, []byte, error) {
	key, err := c.certGroup.GetNewKey()
	if err != nil {
		return nil, nil, err
	}

	return c.Renew(deviceID, key)
}

func (c *ClientCert) WriteCertificate(cert, key []byte) error {
	certGroup := &mtls.CertificateGroup{
		CertPEM:    bytes.NewBuffer(cert),
		PrivKeyPEM: bytes.NewBuffer(key),
	}

	err := certGroup.ImportFromPem()
	if err != nil {
		return fmt.Errorf("failing on importing certificate: %v", err)
	}

	err = os.WriteFile(c.certPath, cert, 0600)
	if err != nil {
		return err
	}

	err = os.WriteFile(c.keyPath, key, 0600)
	if err != nil {
		// Write the cert back because cannot write the new key
		certErr := os.WriteFile(c.certPath, c.certGroup.CertPEM.Bytes(), 0600)
		if certErr != nil {
			log.Error("cannot restore certificate on key write", certErr)
		}
		return err
	}
	c.certGroup = certGroup
	return nil
}
