/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/keikoproj/active-monitor/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GetArtifactReader ---

func TestGetArtifactReader_Inline(t *testing.T) {
	content := "hello"
	loc := &v1alpha1.ArtifactLocation{Inline: &content}
	r, err := GetArtifactReader(loc)
	require.NoError(t, err)
	require.NotNil(t, r)
	b, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, []byte(content), b)
}

func TestGetArtifactReader_URL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("workflow-yaml"))
	}))
	defer srv.Close()

	loc := &v1alpha1.ArtifactLocation{URL: &v1alpha1.URLArtifact{Path: srv.URL, VerifyCert: false}}
	r, err := GetArtifactReader(loc)
	require.NoError(t, err)
	require.NotNil(t, r)
	b, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, []byte("workflow-yaml"), b)
}

func TestGetArtifactReader_UnknownReturnsError(t *testing.T) {
	// Neither Inline nor URL set — File artifact is unimplemented
	loc := &v1alpha1.ArtifactLocation{File: &v1alpha1.FileArtifact{Path: "/some/path"}}
	r, err := GetArtifactReader(loc)
	assert.Nil(t, r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown artifact location")
}

// --- InlineReader ---

func TestNewInlineReader_NilReturnsError(t *testing.T) {
	r, err := NewInlineReader(nil)
	assert.Nil(t, r)
	assert.Error(t, err)
}

func TestNewInlineReader_EmptyStringReturnsError(t *testing.T) {
	empty := ""
	r, err := NewInlineReader(&empty)
	assert.Nil(t, r)
	assert.Error(t, err)
}

func TestInlineReader_Read(t *testing.T) {
	content := "apiVersion: argoproj.io/v1alpha1"
	r, err := NewInlineReader(&content)
	require.NoError(t, err)
	b, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, []byte(content), b)
}

// --- URLReader ---

func TestNewURLReader_NilReturnsError(t *testing.T) {
	r, err := NewURLReader(nil)
	assert.Nil(t, r)
	assert.Error(t, err)
}

func TestURLReader_SuccessfulRead(t *testing.T) {
	body := "workflow-content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	r, err := NewURLReader(&v1alpha1.URLArtifact{Path: srv.URL, VerifyCert: false})
	require.NoError(t, err)
	b, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, []byte(body), b)
}

func TestURLReader_Non200ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	r, err := NewURLReader(&v1alpha1.URLArtifact{Path: srv.URL, VerifyCert: false})
	require.NoError(t, err)
	_, err = r.Read()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestURLReader_NetworkErrorReturnsError(t *testing.T) {
	r, err := NewURLReader(&v1alpha1.URLArtifact{Path: "http://127.0.0.1:1", VerifyCert: false})
	require.NoError(t, err)
	_, err = r.Read()
	assert.Error(t, err)
}

func TestURLReader_VerifyCert_False_AcceptsSelfSigned(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// VerifyCert: false → InsecureSkipVerify: true → should accept self-signed cert
	r, err := NewURLReader(&v1alpha1.URLArtifact{Path: srv.URL, VerifyCert: false})
	require.NoError(t, err)
	b, err := r.Read()
	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), b)
}

func TestURLReader_VerifyCert_True_RejectsSelfSigned(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// VerifyCert: true → InsecureSkipVerify: false → should reject self-signed cert
	r, err := NewURLReader(&v1alpha1.URLArtifact{Path: srv.URL, VerifyCert: true})
	require.NoError(t, err)
	_, err = r.Read()
	require.Error(t, err)
	// The error should mention certificate verification failure
	assert.True(t,
		strings.Contains(err.Error(), "certificate") ||
			strings.Contains(err.Error(), "x509") ||
			strings.Contains(err.Error(), "tls"),
		"expected TLS error, got: %v", err)
}
