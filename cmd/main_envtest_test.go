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
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// newTestEnv creates an envtest.Environment configured for the cmd package.
func newTestEnv(t *testing.T) *envtest.Environment {
	t.Helper()
	return &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "bin", "k8s",
			fmt.Sprintf("1.33.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
}

func TestRunWithEnvtest(t *testing.T) {
	testEnv := newTestEnv(t)

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	defer func() { _ = testEnv.Stop() }()

	// run() with a valid envtest config and a pre-cancelled context should
	// shut down cleanly. The important thing is that it does NOT panic
	// from a nil dynClient (the bug this fix addresses).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = run(ctx, cfg, testOpts())
	if err != nil {
		t.Logf("run() returned (expected for cancelled context): %v", err)
	}
}

func TestDynamicClientFromEnvtestConfig(t *testing.T) {
	testEnv := newTestEnv(t)

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	defer func() { _ = testEnv.Stop() }()

	// Valid config should produce a working dynamic client.
	dynClient, err := dynamic.NewForConfig(cfg)
	require.NoError(t, err)
	assert.NotNil(t, dynClient)
}

func TestDynamicClientWithBrokenConfig(t *testing.T) {
	// dynamic.NewForConfig rarely fails (it doesn't dial on creation),
	// but verify the client is non-nil so the reconciler won't panic.
	cfg := &rest.Config{
		Host: "https://localhost:0",
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, dynClient)
}
