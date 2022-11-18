# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

# Build Geth in a stock Go builder container
FROM golang:1.18-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum/
COPY go.sum /go-ethereum/
RUN cd /go-ethereum && go mod download

ADD . /go-ethereum
RUN cd /go-ethereum && go run build/ci.go install -static ./cmd/geth

# Pull Geth into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/

ADD ./genesis.json ./genesis.json
ARG password=vcn123
ARG privatekey=6208a98d1f31430fa51a37b89a0016b842d8570d7e3da0bac7ca5e11bc96b2f6
RUN echo $password > ~/.accountpassword
RUN echo $privatekey > ~/.privatekey

RUN geth init genesis.json
RUN geth account import --password ~/.accountpassword  ~/.privatekey

ENV address="0x789eeac8071ce0faae85a97cdd83f4677524d74d"
ENV bootnodeId="ad64602a3bdaa584949760514e44ee08137256b4950026f96b2f2a9cba3ca33b3b2f1e648f023beb5ca1218926c3712e0083b4cd2706a4a5e44e8169f35a3034"
ENV bootnodeIp="127.0.0.1"
ENV p2port=30303


EXPOSE 8545 8546
#ENTRYPOINT ["geth"]
#CMD exec geth --bootnodes "enode://$bootnodeId@$bootnodeIp:30301" --networkid="500" --verbosity=4 --rpc --rpcaddr "0.0.0.0" --rpcapi "eth,web3,personal,net,miner,admin,debug,db" --rpccorsdomain "*" --syncmode=full --etherbase $address
CMD exec geth  --bootnodes "enode://$bootnodeId@$bootnodeIp:30301" --networkid="90009" --port $p2port --verbosity=4  --http --http.addr "0.0.0.0" --http.api "eth,web3,personal,net,miner,admin,debug,db,clique" --http.corsdomain "*" --syncmode=full --miner.etherbase $address --mine --allow-insecure-unlock --unlock $address --password ~/.accountpassword --miner.gasprice "0"
  

# Add some metadata labels to help programatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"
