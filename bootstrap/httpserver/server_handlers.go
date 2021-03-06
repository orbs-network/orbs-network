// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package httpserver

import (
	"encoding/json"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-spec/types/go/protocol/client"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
	"net/http"
)

type IndexResponse struct {
	Status      string
	Description string
	Version     config.Version
}

// Serves both index and 404 because router is built that way
func (s *HttpServer) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	data, _ := json.MarshalIndent(IndexResponse{
		Status:      "OK",
		Description: "ORBS blockchain public API",
		Version:     config.GetVersion(),
	}, "", "  ")

	_, err := w.Write(data)
	if err != nil {
		s.logger.Info("error writing index.json response", log.Error(err))
	}
}

func (s *HttpServer) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, err := w.Write([]byte("User-agent: *\nDisallow: /\n"))
	if err != nil {
		s.logger.Info("error writing robots.txt response", log.Error(err))
	}
}

func (s *HttpServer) filterOn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, f := range s.logger.Filters() {
		if c, ok := f.(log.ConditionalFilter); ok {
			c.On()
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("filter on"))
}

func (s *HttpServer) filterOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, f := range s.logger.Filters() {
		if c, ok := f.(log.ConditionalFilter); ok {
			c.Off()
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("filter off"))
}

func (s *HttpServer) dumpMetricsAsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bytes, _ := json.Marshal(s.metricRegistry.ExportAllNested(s.logger))
	_, err := w.Write(bytes)
	if err != nil {
		s.logger.Info("error writing response", log.Error(err))
	}
}

func (s *HttpServer) dumpMetricsAsPrometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	prometheusMetrics := s.metricRegistry.ExportPrometheus()

	_, err := w.Write([]byte(prometheusMetrics))
	if err != nil {
		s.logger.Info("error writing response", log.Error(err))
	}
}

func (s *HttpServer) sendTransactionHandler(w http.ResponseWriter, r *http.Request) {
	bytes, e := readInput(r)
	if e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	clientRequest := client.SendTransactionRequestReader(bytes)
	if e := validate(clientRequest); e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	s.logger.Info("http HttpServer received send-transaction", log.Stringable("request", clientRequest))
	result, err := s.publicApi.SendTransaction(r.Context(), &services.SendTransactionInput{ClientRequest: clientRequest})
	if result != nil && result.ClientResponse != nil {
		s.writeMembuffResponse(w, result.ClientResponse, result.ClientResponse.RequestResult(), err)
	} else {
		s.writeErrorResponseAndLog(w, &httpErr{http.StatusInternalServerError, log.Error(err), err.Error()})
	}
}

func (s *HttpServer) sendTransactionAsyncHandler(w http.ResponseWriter, r *http.Request) {
	bytes, e := readInput(r)
	if e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	clientRequest := client.SendTransactionRequestReader(bytes)
	if e := validate(clientRequest); e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	s.logger.Info("http HttpServer received send-transaction-async", log.Stringable("request", clientRequest))
	result, err := s.publicApi.SendTransactionAsync(r.Context(), &services.SendTransactionInput{ClientRequest: clientRequest})
	if result != nil && result.ClientResponse != nil {
		s.writeMembuffResponse(w, result.ClientResponse, result.ClientResponse.RequestResult(), err)
	} else {
		s.writeErrorResponseAndLog(w, &httpErr{http.StatusInternalServerError, log.Error(err), err.Error()})
	}
}

func (s *HttpServer) runQueryHandler(w http.ResponseWriter, r *http.Request) {
	bytes, e := readInput(r)
	if e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	clientRequest := client.RunQueryRequestReader(bytes)
	if e := validate(clientRequest); e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	s.logger.Info("http HttpServer received run-query", log.Stringable("request", clientRequest))
	result, err := s.publicApi.RunQuery(r.Context(), &services.RunQueryInput{ClientRequest: clientRequest})
	if result != nil && result.ClientResponse != nil {
		s.writeMembuffResponse(w, result.ClientResponse, result.ClientResponse.RequestResult(), err)
	} else {
		s.writeErrorResponseAndLog(w, &httpErr{http.StatusInternalServerError, log.Error(err), err.Error()})
	}
}

func (s *HttpServer) getTransactionStatusHandler(w http.ResponseWriter, r *http.Request) {
	bytes, e := readInput(r)
	if e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	clientRequest := client.GetTransactionStatusRequestReader(bytes)
	if e := validate(clientRequest); e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	s.logger.Info("http HttpServer received get-transaction-status", log.Stringable("request", clientRequest))
	result, err := s.publicApi.GetTransactionStatus(r.Context(), &services.GetTransactionStatusInput{ClientRequest: clientRequest})
	if result != nil && result.ClientResponse != nil {
		s.writeMembuffResponse(w, result.ClientResponse, result.ClientResponse.RequestResult(), err)
	} else {
		s.writeErrorResponseAndLog(w, &httpErr{http.StatusInternalServerError, log.Error(err), err.Error()})
	}
}

func (s *HttpServer) getTransactionReceiptProofHandler(w http.ResponseWriter, r *http.Request) {
	bytes, e := readInput(r)
	if e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	clientRequest := client.GetTransactionReceiptProofRequestReader(bytes)
	if e := validate(clientRequest); e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	s.logger.Info("http HttpServer received get-transaction-receipt-proof", log.Stringable("request", clientRequest))
	result, err := s.publicApi.GetTransactionReceiptProof(r.Context(), &services.GetTransactionReceiptProofInput{ClientRequest: clientRequest})
	if result != nil && result.ClientResponse != nil {
		s.writeMembuffResponse(w, result.ClientResponse, result.ClientResponse.RequestResult(), err)
	} else {
		s.writeErrorResponseAndLog(w, &httpErr{http.StatusInternalServerError, log.Error(err), err.Error()})
	}
}

func (s *HttpServer) getBlockHandler(w http.ResponseWriter, r *http.Request) {
	bytes, e := readInput(r)
	if e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	clientRequest := client.GetBlockRequestReader(bytes)
	if e := validate(clientRequest); e != nil {
		s.writeErrorResponseAndLog(w, e)
		return
	}

	s.logger.Info("http HttpServer received get-block", log.Stringable("request", clientRequest))
	result, err := s.publicApi.GetBlock(r.Context(), &services.GetBlockInput{ClientRequest: clientRequest})
	if result != nil && result.ClientResponse != nil {
		s.writeMembuffResponse(w, result.ClientResponse, result.ClientResponse.RequestResult(), err)
	} else {
		s.writeErrorResponseAndLog(w, &httpErr{http.StatusInternalServerError, log.Error(err), err.Error()})
	}
}
