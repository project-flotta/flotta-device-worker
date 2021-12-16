package s3

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"

	"git.sr.ht/~spc/go-log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jakub-dzon/k4e-operator/models"
	"github.com/seqsense/s3sync"
)

var (
	theTrue = true
)

type Sync struct {
	s3SyncManager  *s3sync.Manager
	bucketName     string
	bucketRegion   string
	endpoint       string
	accessKeyBytes []byte
	secretKeyBytes []byte
	caBundle       []byte
}

func NewSync(s3Config models.S3StorageConfiguration) (*Sync, error) {

	sync := Sync{}
	var err error

	sync.accessKeyBytes, err = base64.StdEncoding.DecodeString(s3Config.AwsAccessKeyID)
	if err != nil {
		log.Errorf("cannot decode AWS Access Key: %v", err)
		return nil, err
	}
	sync.secretKeyBytes, err = base64.StdEncoding.DecodeString(s3Config.AwsSecretAccessKey)
	if err != nil {
		log.Errorf("cannot decode AWS Access Key: %v", err)
		return nil, err
	}

	sync.caBundle, err = base64.StdEncoding.DecodeString(s3Config.AwsCaBundle)
	if err != nil {
		log.Errorf("cannot decode AWS CA Bundle: %v", err)
		return nil, err
	}

	sync.endpoint = fmt.Sprintf("https://%s:%d", s3Config.BucketHost, s3Config.BucketPort)
	sync.bucketName = s3Config.BucketName
	sync.bucketRegion = s3Config.BucketRegion
	return &sync, nil
}

func (s *Sync) Connect() error {
	if s.s3SyncManager != nil {
		return nil
	}

	sess, err := session.NewSession(&aws.Config{
		Region:           &s.bucketRegion,
		Endpoint:         &s.endpoint,
		Credentials:      credentials.NewStaticCredentials(string(s.accessKeyBytes), string(s.secretKeyBytes), ""),
		HTTPClient:       createHttpClient(s.caBundle),
		S3ForcePathStyle: &theTrue,
	})

	if err != nil {
		log.Errorf("error while creating s3 session: %v", err)
		return err
	}

	s.s3SyncManager = s3sync.New(sess)
	return nil
}

func createHttpClient(caBundle []byte) *http.Client {
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	rootCAs.AppendCertsFromPEM(caBundle)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: rootCAs},
	}
	client := &http.Client{Transport: tr}
	return client
}

func (s *Sync) SyncPath(sourcePath, targetPath string) error {
	target := fmt.Sprintf("s3://%s/%s", s.bucketName, targetPath)
	return s.s3SyncManager.Sync(sourcePath, target)
}
