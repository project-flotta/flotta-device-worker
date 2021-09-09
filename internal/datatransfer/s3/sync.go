package s3

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"git.sr.ht/~spc/go-log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jakub-dzon/k4e-operator/models"
	"github.com/seqsense/s3sync"
	"net/http"
)

var (
	theTrue      = true
	ignoreRegion = "ignore"
)

type Sync struct {
	s3SyncManager *s3sync.Manager
	bucketName    string
}

func NewSync(s3Config models.S3StorageConfiguration) (*Sync, error) {
	accessKeyBytes, err := base64.StdEncoding.DecodeString(s3Config.AwsAccessKeyID)
	if err != nil {
		log.Errorf("Can't decode AWS Access Key: %v", err)
	}
	secretKeyBytes, err := base64.StdEncoding.DecodeString(s3Config.AwsSecretAccessKey)
	if err != nil {
		log.Errorf("Can't decode AWS Access Key: %v", err)
	}
	caBundle, err := base64.StdEncoding.DecodeString(s3Config.AwsCaBundle)
	if err != nil {
		log.Errorf("Can't decode AWS CA Bundle: %v", err)
	}

	endpoint := fmt.Sprintf("https://%s:%d", s3Config.BucketHost, s3Config.BucketPort)
	sess, err := session.NewSession(&aws.Config{
		Region:           &ignoreRegion,
		Endpoint:         &endpoint,
		Credentials:      credentials.NewStaticCredentials(string(accessKeyBytes), string(secretKeyBytes), ""),
		HTTPClient:       createHttpClient(caBundle),
		S3ForcePathStyle: &theTrue,
	})
	if err != nil {
		log.Errorf("Error while creating s3 session: %v", err)
		return nil, err
	}
	return &Sync{
		s3SyncManager: s3sync.New(sess),
		bucketName:    s3Config.BucketName,
	}, nil
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
