package acceptance

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/orbs-network/orbs-network-go/test/harness"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
)

var _ = Describe("a leader node", func() {

	It("commits transactions to all nodes, skipping invalid transactions", func() {
		// leader is nodeIndex 0, validator is nodeIndex 1
		network := harness.NewTestNetwork(2)
		defer network.FlushLog()

		network.SendTransfer(0, 17)
		network.SendTransfer(0, 1000000) //this is invalid because currently we don't allow (temporarily) values over 1000 just so that we can create invalid transactions
		network.SendTransfer(0, 22)

		network.BlockPersistence(0).WaitForBlocks(2)
		Expect(<-network.CallGetBalance(0)).To(BeEquivalentTo(39))

		network.BlockPersistence(1).WaitForBlocks(2)
		Expect(<-network.CallGetBalance(1)).To(BeEquivalentTo(39))
	})

})

var _ = Describe("a non-leader (validator) node", func() {

	It("propagates transactions to leader but does not commit them itself", func() {
		// leader is nodeIndex 0, validator is nodeIndex 1
		network := harness.NewTestNetwork(2)

		network.GossipTransport().Pause(gossipmessages.HEADER_TOPIC_TRANSACTION_RELAY, uint16(gossipmessages.TRANSACTION_RELAY_FORWARDED_TRANSACTIONS))
		network.SendTransfer(1, 17)

		Expect(<-network.CallGetBalance(0)).To(BeEquivalentTo(0))
		Expect(<-network.CallGetBalance(1)).To(BeEquivalentTo(0))

		network.GossipTransport().Resume(gossipmessages.HEADER_TOPIC_TRANSACTION_RELAY, uint16(gossipmessages.TRANSACTION_RELAY_FORWARDED_TRANSACTIONS))
		network.BlockPersistence(0).WaitForBlocks(1)
		Expect(<-network.CallGetBalance(0)).To(BeEquivalentTo(17))
		network.BlockPersistence(1).WaitForBlocks(1)
		Expect(<-network.CallGetBalance(1)).To(BeEquivalentTo(17))
	})

})
