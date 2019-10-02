// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package fake

import (
	"context"
	sdkContext "github.com/orbs-network/orbs-contract-sdk/go/context"
	"github.com/orbs-network/orbs-network-go/services/processor/native/adapter"
	"github.com/pkg/errors"
	"strings"
	"sync"
)

type FakeCompiler interface {
	adapter.Compiler
	// Does not support multi-file fakes!
	ProvideFakeContract(fakeContractInfo *sdkContext.ContractInfo, code ...string)
}

type fakeCompiler struct {
	mutex    *sync.RWMutex
	provided map[string]*sdkContext.ContractInfo
}

func NewCompiler() *fakeCompiler {
	return &fakeCompiler{
		mutex:    &sync.RWMutex{},
		provided: make(map[string]*sdkContext.ContractInfo),
	}
}

func (c *fakeCompiler) ProvideFakeContract(fakeContractInfo *sdkContext.ContractInfo, code ...string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if len(code) > 1 {
		panic("fake compiler does not support multiple files in contracts")
	}

	c.provided[strings.TrimSpace(code[0])] = fakeContractInfo
}

func (c *fakeCompiler) Compile(ctx context.Context, code ...string) (*sdkContext.ContractInfo, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	contractInfo, found := c.provided[strings.TrimSpace(code[0])]
	if !found {
		return nil, errors.New("fake contract for given code was not previously provided with ProvideFakeContract()")
	}

	return contractInfo, nil
}
