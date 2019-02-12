package publicapi

import (
	"context"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"time"
)

var LogTag = log.Service("public-api")

type service struct {
	config          config.PublicApiConfig
	transactionPool services.TransactionPool
	virtualMachine  services.VirtualMachine
	blockStorage    services.BlockStorage
	logger          log.BasicLogger

	waiter *waiter

	metrics *metrics
}

type metrics struct {
	sendTransactionTime      *metric.Histogram
	getTransactionStatusTime *metric.Histogram
	runQueryTime             *metric.Histogram
}

func newMetrics(factory metric.Factory, sendTransactionTimeout time.Duration, getTransactionStatusTimeout time.Duration, runQueryTimeout time.Duration) *metrics {
	return &metrics{
		sendTransactionTime:      factory.NewLatency("PublicApi.SendTransactionProcessingTime", sendTransactionTimeout),
		getTransactionStatusTime: factory.NewLatency("PublicApi.GetTransactionStatusProcessingTime", getTransactionStatusTimeout),
		runQueryTime:             factory.NewLatency("PublicApi.RunQueryProcessingTime", runQueryTimeout),
	}
}

func NewPublicApi(
	config config.PublicApiConfig,
	transactionPool services.TransactionPool,
	virtualMachine services.VirtualMachine,
	blockStorage services.BlockStorage,
	logger log.BasicLogger,
	metricFactory metric.Factory,
) services.PublicApi {
	s := &service{
		config:          config,
		transactionPool: transactionPool,
		virtualMachine:  virtualMachine,
		blockStorage:    blockStorage,
		logger:          logger.WithTags(LogTag),

		waiter:  newWaiter(),
		metrics: newMetrics(metricFactory, config.PublicApiSendTransactionTimeout(), 2*time.Second, 1*time.Second),
	}

	transactionPool.RegisterTransactionResultsHandler(s)

	return s
}

func (s *service) HandleTransactionResults(ctx context.Context, input *handlers.HandleTransactionResultsInput) (*handlers.HandleTransactionResultsOutput, error) {
	for _, txReceipt := range input.TransactionReceipts {
		s.waiter.complete(txReceipt.Txhash().KeyForMap(),
			&txOutput{
				transactionStatus:  protocol.TRANSACTION_STATUS_COMMITTED,
				transactionReceipt: txReceipt,
				blockHeight:        input.BlockHeight,
				blockTimestamp:     input.Timestamp,
			})
	}
	return &handlers.HandleTransactionResultsOutput{}, nil
}

func (s *service) HandleTransactionError(ctx context.Context, input *handlers.HandleTransactionErrorInput) (*handlers.HandleTransactionErrorOutput, error) {
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx), log.String("flow", "checkpoint"))

	logger.Info("transaction reported as erred", log.Transaction(input.Txhash), log.Stringable("tx-status", input.TransactionStatus))
	s.waiter.complete(input.Txhash.KeyForMap(),
		&txOutput{
			transactionStatus:  input.TransactionStatus,
			transactionReceipt: nil,
			blockHeight:        input.BlockHeight,
			blockTimestamp:     input.BlockTimestamp,
		})
	return &handlers.HandleTransactionErrorOutput{}, nil
}
