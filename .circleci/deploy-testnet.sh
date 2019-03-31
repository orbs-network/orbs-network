#!/bin/bash -e

# Installing aws cli
echo "Installing AWS CLI"
sudo apt-get update
sudo apt-get install -y python-dev
sudo apt-get install -y python-pip
sudo pip install awscli

aws --version

touch $BASH_ENV
curl -o- https://raw.githubusercontent.com/creationix/nvm/v0.33.11/install.sh | bash
export NVM_DIR="/opt/circleci/.nvm" && . $NVM_DIR/nvm.sh && nvm install v10.14.1 && nvm use v10.14.1
export COMMIT_HASH=$(./docker/hash.sh)

curl -O https://s3.eu-central-1.amazonaws.com/boyar-ci/boyar/config.json
node .circleci/testnet-deploy-tag.js $COMMIT_HASH

aws s3 cp --acl public-read config.json s3://boyar-ci/boyar/config.json

echo "Configuration updated for all nodes in the CI testnet"
echo "Waiting for all nodes to restart and reflect the new version is running"

sleep 20

node .circleci/check-testnet-deployment.js $COMMIT_HASH