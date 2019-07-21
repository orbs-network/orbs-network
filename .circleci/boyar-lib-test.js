const { describe, it } = require('mocha');
const { expect } = require('chai');

const { isPortUnique, newVacantTCPPort } = require('./boyar-lib');

describe('boyar library tests', () => {
    it('should identify a port as unique correctly', () => {
        const configuration = {
            chains: [
                {
                    HttpPort: 1000,
                    GossipPort: 1001,
                },
                {
                    HttpPort: 1003,
                    GossipPort: 1005,
                },
            ]
        }

        expect(isPortUnique(configuration, 1002)).to.equal(true)
        expect(isPortUnique(configuration, 1000)).to.equal(false)
    })

    it('should provide a vacant port', () => {
        const configuration = {
            chains: [
                {
                    HttpPort: 1000,
                    GossipPort: 1001,
                },
                {
                    HttpPort: 1003,
                    GossipPort: 1005,
                },
            ]
        }

        const newPort = newVacantTCPPort(configuration)
        expect([1000,1001,1003,1005]).to.not.include(newPort)
    })
})
