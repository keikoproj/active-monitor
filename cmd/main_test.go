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

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

// testOpts returns managerOptions suitable for testing (ephemeral ports, no auth).
func testOpts() managerOptions {
	return managerOptions{
		metricsAddr: ":0",
		probeAddr:   ":0",
		maxParallel: 1,
	}
}

func TestRunReturnsErrorForInvalidManager(t *testing.T) {
	// Invalid TLS cert data causes the manager (REST client) to fail.
	cfg := &rest.Config{
		Host: "https://localhost:1",
		TLSClientConfig: rest.TLSClientConfig{
			CertData: []byte("not-a-cert"),
			KeyData:  []byte("not-a-key"),
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := run(ctx, cfg, testOpts())
	assert.Error(t, err, "run() should return an error for invalid TLS config")
}

func TestRunShutdownOnCancelledContext(t *testing.T) {
	// With a valid-looking config but cancelled context, run() should
	// return cleanly (possibly nil, possibly context-cancelled error).
	// This test primarily verifies no panic occurs — the core assertion
	// for the bug fix (nil dynClient would have panicked before the fix).
	cfg := &rest.Config{
		Host: "https://127.0.0.1:0",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should not panic.
	_ = run(ctx, cfg, testOpts())
}
