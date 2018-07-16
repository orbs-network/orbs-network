package test

import (
	"github.com/maraino/go-mock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/orbs-network/orbs-network-go/services/transactionpool"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/harness/instrumentation"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
)

var _ = Describe("transaction pool", func() {

	var (
		gossip  *gossiptopics.MockTransactionRelay
		service services.TransactionPool
	)

	BeforeEach(func() {
		log := instrumentation.NewBufferedLog("TransactionPool")
		gossip = &gossiptopics.MockTransactionRelay{}
		gossip.When("RegisterTransactionRelayHandler", mock.Any).Return()
		service = transactionpool.NewTransactionPool(gossip, log)
	})

	It("forwards a new valid transaction with gossip", func() {

		tx := test.TransferTransaction().Build()

		gossip.When("BroadcastForwardedTransactions", &gossiptopics.ForwardedTransactionsInput{
			Message: &gossipmessages.ForwardedTransactionsMessage{
				SignedTransactions: []*protocol.SignedTransaction{tx},
			},
		}).Return(&gossiptopics.EmptyOutput{}, nil).Times(1)

		_, err := service.AddNewTransaction(&services.AddNewTransactionInput{
			SignedTransaction: tx,
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(gossip).To(test.ExecuteAsPlanned())

	})

})
