// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package signer

import (
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
	"io/ioutil"
	"net/http"
)

type api struct {
	vault  services.Vault
	logger log.Logger
}

func (a *api) SignHandler(writer http.ResponseWriter, request *http.Request) {
	input, err := ioutil.ReadAll(request.Body)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		a.logger.Error("failed to read request body")
		return
	}

	ctx := request.Context()
	ctx = trace.NewFromRequest(ctx, request)

	if signature, err := a.vault.NodeSign(ctx, services.NodeSignInputReader(input)); err == nil {
		a.logger.Info("successfully signed payload")
		if _, err := writer.Write(signature.Raw()); err != nil {
			a.logger.Error("could not write response body into the socket", log.Error(err))
		}

		return
	}

	writer.WriteHeader(http.StatusInternalServerError)
}
