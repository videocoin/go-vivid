// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// The implementaiton currently covers consesnus for dynamiuc fee agreement drvien by predection from light workers
// This module can be extended to cover ohter governance items in futue
//
package clique

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru"
)

type FeeVote struct {
	Signer common.Address `json:"signer"` // Authorized signer that cast this vote
	Block  uint64         `json:"block"`  // Block number the vote was cast in (expire old votes)
	Fee    int32          `json:"fee"`
}

// Snapshot is the state of the authorization voting at a given point in time.
type FeeSnapshot struct {
	config   *params.CliqueConfig // Consensus engine parameters to fine tune behavior
	sigcache *lru.ARCCache        // Cache of recent block signatures to speed up ecrecover

	Number   uint64                   `json:"number"` // Block number where the snapshot was created
	Hash     common.Hash              `json:"hash"`   // Block hash where the snapshot was created
	Tally    map[common.Address]int32 `json:"tally"`  // Current vote tally to avoid recalculating
	FeeVotes []*FeeVote               `json:"freeProposals"`
}

// newSnapshot creates a new snapshot with the specified startup parameters. This
// method does not initialize the set of recent signers, so only ever use if for
// the genesis block.
func newFeeSnapshot(config *params.CliqueConfig, sigcache *lru.ARCCache, number uint64, hash common.Hash, signers []common.Address) *Snapshot {
	snap := &Snapshot{
		config: config,
		Number: number,
		Hash:   hash,
	}

	return snap
}

// loadSnapshot loads an existing snapshot from the database.
func loadFeeSnapshot(config *params.CliqueConfig, sigcache *lru.ARCCache, db ethdb.Database, hash common.Hash) (*FeeSnapshot, error) {
	blob, err := db.Get(append([]byte("clique-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	snap := new(FeeSnapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config
	snap.sigcache = sigcache

	return snap, nil
}

// store inserts the snapshot into the database.
func (s *FeeSnapshot) store(db ethdb.Database) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("clique-"), s.Hash[:]...), blob)
}

// copy creates a deep copy of the snapshot, though not the individual votes.
func (s *FeeSnapshot) copy() *FeeSnapshot {
	cpy := &FeeSnapshot{
		config:   s.config,
		sigcache: s.sigcache,
		Number:   s.Number,
		Hash:     s.Hash,
		FeeVotes: make([]*FeeVote, len(s.FeeVotes)),
	}

	for address, tally := range s.Tally {
		cpy.Tally[address] = tally
	}
	copy(cpy.FeeVotes, s.FeeVotes)
	return cpy
}

// cast adds a new vote into the tally.
func (s *FeeSnapshot) cast(signer common.Address, fee int32) bool {
	s.Tally[signer] = fee
	return true
}

// apply creates a new authorization snapshot by applying the given headers to
// the original one.
func (s *FeeSnapshot) apply(headers []*types.Header, signers []common.Address) (*FeeSnapshot, int, error) {
	// Allow passing in no headers for cleaner code
	agreedFee := 0
	if len(headers) == 0 {
		return s, agreedFee, nil
	}
	// Sanity check that the headers can be applied
	for i := 0; i < len(headers)-1; i++ {
		if headers[i+1].Number.Uint64() != headers[i].Number.Uint64()+1 {
			return nil, agreedFee, errInvalidVotingChain
		}
	}
	if headers[0].Number.Uint64() != s.Number+1 {
		return nil, agreedFee, errInvalidVotingChain
	}
	// Iterate through the headers and create a new snapshot
	snap := s.copy()

	var (
		start  = time.Now()
		logged = time.Now()
	)
	for i, header := range headers {
		// Remove any votes on checkpoint blocks
		number := header.Number.Uint64()

		// Resolve the authorization key and check against signers
		signer, err := ecrecover(header, s.sigcache)
		if err != nil {
			return nil, agreedFee, err
		}

		var fee int32
		agreeCount := 0

		switch {
		case bytes.Equal(header.Nonce[0:4], nonceDropVote):
			fee = int32(binary.LittleEndian.Uint32(header.Nonce[4:8]))
		default:
			return nil, agreedFee, errInvalidFeeVote
		}
		snap.FeeVotes = append(snap.FeeVotes, &FeeVote{
			Signer: signer,
			Block:  number,
			Fee:    fee,
		})
		// If the vote passed, update the fee
		if len(snap.Tally) > len(signers)/2 {
			// Discard any previous votes around the just changed account
			for i := 0; i < len(snap.FeeVotes); i++ {
				if snap.FeeVotes[i].Fee == fee /* TODO*/ {
					agreeCount++
				}
			}
		}
		if agreeCount < len(signers)/2 {
			agreedFee = int(fee)
		}
		// If we're taking too much time (ecrecover), notify the user once a while
		if time.Since(logged) > 8*time.Second {
			log.Info("Reconstructing voting history", "processed", i, "total", len(headers), "elapsed", common.PrettyDuration(time.Since(start)))
			logged = time.Now()
		}
	}
	if time.Since(start) > 8*time.Second {
		log.Info("Reconstructed voting history", "processed", len(headers), "elapsed", common.PrettyDuration(time.Since(start)))
	}
	snap.Number += uint64(len(headers))
	snap.Hash = headers[len(headers)-1].Hash()

	return snap, agreedFee, nil
}
