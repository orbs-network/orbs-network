// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.
//+build !race

package test

import (
	"context"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/processor/native/adapter"
	"github.com/orbs-network/orbs-network-go/services/processor/native/adapter/fake"
	"github.com/orbs-network/orbs-network-go/services/processor/native/types"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/contracts"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/scribe/log"
	"github.com/stretchr/testify/require"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestContract_Compile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compilation of contracts in short mode")
	}

	t.Run("FakeCompiler", compileTest(aFakeCompiler))
	t.Run("NativeCompiler", compileTest(aNativeCompiler))
}

func compileTest(newHarness func(t *testing.T, logger log.Logger) *compilerContractHarness) func(*testing.T) {
	return func(t *testing.T) {
		with.Logging(t, func(parent *with.LoggingHarness) {
			h := newHarness(t, parent.Logger)
			defer h.cleanup()

			// give the test one minute timeout to compile
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			t.Log("Compiling a valid contract")

			code := string(contracts.NativeSourceCodeForCounter(contracts.MOCK_COUNTER_CONTRACT_START_FROM))
			compilationStartTime := time.Now().UnixNano()
			contractInfo, err := h.compiler.Compile(ctx, code)
			compilationTimeMs := (time.Now().UnixNano() - compilationStartTime) / 1000000
			t.Logf("Compilation time: %d ms", compilationTimeMs)

			require.NoError(t, err, "compilation should succeed")
			require.NotNil(t, contractInfo, "loaded object should not be nil")

			codePart1 := string(contracts.NativeSourceCodeForCounterPart1(contracts.MOCK_COUNTER_CONTRACT_START_FROM))
			codePart2 := string(contracts.NativeSourceCodeForCounterPart2(contracts.MOCK_COUNTER_CONTRACT_START_FROM))
			compilationStartTime = time.Now().UnixNano()
			contractInfo, err = h.compiler.Compile(ctx, codePart1, codePart2)
			compilationTimeMs = (time.Now().UnixNano() - compilationStartTime) / 1000000
			t.Logf("Compilation time: %d ms", compilationTimeMs)

			require.NoError(t, err, "compilation of multiple files should succeed")
			require.NotNil(t, contractInfo, "loaded object should not be nil")

			// instantiate the "start()" function of the contract and call it
			contractInstance, err := types.NewContractInstance(contractInfo)
			require.NoError(t, err, "create contract instance should succeed")
			res := reflect.ValueOf(contractInstance.PublicMethods["start"]).Call([]reflect.Value{})
			require.Equal(t, contracts.MOCK_COUNTER_CONTRACT_START_FROM, res[0].Interface().(uint64), "result of calling start() should match")

			t.Log("Compiling an invalid contract")

			invalidCode := "invalid code example"
			_, err = h.compiler.Compile(ctx, invalidCode)
			require.Error(t, err, "compile should fail")
		})

	}
}

type compilerContractHarness struct {
	compiler adapter.Compiler
	cleanup  func()
}

func aNativeCompiler(t *testing.T, logger log.Logger) *compilerContractHarness {
	tmpDir := test.CreateTempDirForTest(t)
	cfg := &hardcodedConfig{artifactPath: tmpDir, maxWarmupTime: 15 * time.Second}
	compiler := adapter.NewNativeCompiler(cfg, logger, metric.NewRegistry())
	return &compilerContractHarness{
		compiler: compiler,
		cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	}
}

func TestNativeCompilerPanicOnWarmupCompilationFailure(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Warmup compilation did not panic due to short allowed compilation time")
		}
	}()

	with.Logging(t, func(parent *with.LoggingHarness) {
		parent.AllowErrorsMatching("deadline exceeded")
		tmpDir := test.CreateTempDirForTest(t)
		cfg := &hardcodedConfig{artifactPath: tmpDir, maxWarmupTime: 20 * time.Nanosecond}
		adapter.NewNativeCompiler(cfg, parent.Logger, metric.NewRegistry())
	})
}

func aFakeCompiler(t *testing.T, logger log.Logger) *compilerContractHarness {
	compiler := fake.NewCompiler()
	code := string(contracts.NativeSourceCodeForCounter(contracts.MOCK_COUNTER_CONTRACT_START_FROM))
	compiler.ProvideFakeContract(contracts.MockForCounter(), code)

	codePart1 := string(contracts.NativeSourceCodeForCounterPart1(contracts.MOCK_COUNTER_CONTRACT_START_FROM))
	compiler.ProvideFakeContract(contracts.MockForCounter(), codePart1)

	return &compilerContractHarness{
		compiler: compiler,
		cleanup:  func() {},
	}
}

type hardcodedConfig struct {
	artifactPath  string
	maxWarmupTime time.Duration
}

func (c *hardcodedConfig) MaxWarmupCompilationTime() time.Duration {
	return c.maxWarmupTime
}

func (c *hardcodedConfig) ProcessorPerformWarmUpCompilation() bool {
	return true
}

func (c *hardcodedConfig) ProcessorArtifactPath() string {
	return c.artifactPath
}
