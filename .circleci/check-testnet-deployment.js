#!/usr/bin/env node
const path = require('path');

const { waitUntilSync, waitUntilCommit, getBlockHeight } = require('@orbs-network/orbs-nebula/lib/metrics');

const configFilePath = path.join(process.cwd(), 'config.json');
const topology = require(configFilePath);

const targetChainId = process.argv[2];

if (!targetChainId) {
    console.log('No chainId given!');
    process.exit(1);
}

async function eventuallyDeployed({ chainId, nodes }) {
    // The correct hash for this chainId is..
    const chain = topology.chains.find(chain => chain.Id === chainId);
    const chainSpecificTargetHash = chain.DockerConfig.Tag;

    // First let's poll the nodes for the correct version
    let versionDeployed = false;

    const promises = nodes.map(({ ip }) => {
        console.log('waiting until commit for chain id: ', chainId, ' and IP: ', ip, ' and commit: ', chainSpecificTargetHash);
        return waitUntilCommit(`${ip}/vchains/${chainId}`, chainSpecificTargetHash);
    });

    try {
        await Promise.all(promises);
        versionDeployed = true;
    } catch (err) {
        console.log(`Version ${chainSpecificTargetHash} might not be deployed on all CI testnet nodes!`);
        console.log('error provided:', err);
    }

    return {
        ok: versionDeployed
    };
}

async function eventuallyClosingBlocks({ chainId, nodes }) {
    const firstEndpoint = `${nodes[0].ip}/vchains/${chainId}`;

    // First let's get the current blockheight and wait for it to close 5 more blocks
    const currentBlockheight = await getBlockHeight(firstEndpoint);
    console.log('Fetching current blockheight of the network: ', currentBlockheight);

    try {
        await waitUntilSync(firstEndpoint, currentBlockheight + 5);

        return {
            ok: true,
            chainId
        };
    } catch (err) {
        console.log('Network is not advancing for vchain: ', chainId, ' with error: ', err);
        return err;
    }
}

(async () => {
    const nodes = topology.network.filter(({ ip }) => ip !== '54.149.67.22');
    const chains = topology.chains.map(chain => chain.Id).filter(chainId => chainId === parseInt(targetChainId));

    if (chains.length === 0) {
        console.log('No chains to check!');
        process.exit(2);
    }

    const results = await Promise.all(chains.map((chainId) => eventuallyDeployed({ chainId, nodes })));
    if (results.filter(r => r.ok === true).length === chains.length) {
        console.log('New version deployed successfully on all chains in the testnet');
    } else {
        console.error('New version was not deployed on all nodes within the defined 15 minutes window, quiting..');
        process.exit(2);
    }

    const cbResults = await Promise.all(chains.map((chainId) => eventuallyClosingBlocks({ chainId, nodes })));
    if (cbResults.filter(r => r.ok === true).length === chains.length) {
        console.log('Blocks are being closed on all chains in the testnet!');
        process.exit(0);
    } else {
        console.error('Not all chains are closing blocks after the new version was deployed within the defined 15 minutes window, quiting..');
        process.exit(3);
    }
})();