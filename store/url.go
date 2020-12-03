package store

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/keikoproj/active-monitor/api/v1alpha1"
	"github.com/sirupsen/logrus"
)

// URLReader implements the ArtifactReader interface for urls
type URLReader struct {
	urlArtifact *v1alpha1.URLArtifact
}

// NewURLReader creates a new ArtifactReader for workflows at URL endpoints.
func NewURLReader(urlArtifact *v1alpha1.URLArtifact) (ArtifactReader, error) {
	if urlArtifact == nil {
		return nil, errors.New("URLArtifact cannot be empty")
	}
	return &URLReader{urlArtifact}, nil
}

func (reader *URLReader) Read() ([]byte, error) {
	logrus.Debugf("reading urlArtifact from %s", reader.urlArtifact.Path)
	insecureSkipVerify := !reader.urlArtifact.VerifyCert
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		},
	}
	resp, err := client.Get(reader.urlArtifact.Path)
	if err != nil {
		logrus.Warnf("failed to read url %s: %s", reader.urlArtifact.Path, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logrus.Warnf("failed to read %s. status code: %d", reader.urlArtifact.Path, resp.StatusCode)
		return nil, errors.New("status code " + strconv.Itoa(resp.StatusCode))
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warnf("failed to read url body for %s: %s", reader.urlArtifact.Path, err)
		return nil, err
	}
	return content, nil
}
