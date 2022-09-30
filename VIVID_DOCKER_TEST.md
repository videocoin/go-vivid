docker build -t geth-test .
docker run -it -p 8545:8545 --name vivid geth-test

docker exec -it vivid /bin/sh

# shell in vivid container
# https://geth.ethereum.org/docs/interface/javascript-console
geth attach

# geth console. Check some commands
eth.accounts[0]
clique.status()

