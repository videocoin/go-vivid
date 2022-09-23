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
ENV bootnodeId=""
ENV bootnodeIp=""

EXPOSE 8545 8546 30303 30303/udp
#ENTRYPOINT ["geth"]
#CMD exec geth --bootnodes "enode://$bootnodeId@$bootnodeIp:30301" --networkid="500" --verbosity=4 --rpc --rpcaddr "0.0.0.0" --rpcapi "eth,web3,personal,net,miner,admin,debug,db" --rpccorsdomain "*" --syncmode=full --etherbase $address
CMD exec geth  --networkid="90009" --verbosity=4  --http --http.addr "0.0.0.0" --http.api "eth,web3,personal,net,miner,admin,debug,db" --http.corsdomain "*" --syncmode=full --miner.etherbase $address --mine --allow-insecure-unlock --unlock $address --password ~/.accountpassword --miner.gasprice "0"


# Add some metadata labels to help programatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"
