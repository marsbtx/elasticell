// Copyright 2016 DeepFabric, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package pdserver

import (
	"sync"

	"github.com/deepfabric/elasticell/pkg/log"
	"github.com/deepfabric/elasticell/pkg/pb/metapb"
	"github.com/pkg/errors"
)

const (
	batchLimit = 10000
)

func newCache(clusterID uint64, store Store, allocator *idAllocator) *cache {
	c := new(cache)
	c.clusterID = clusterID
	c.sc = newStoreCache()
	c.cc = newCellCache()
	c.store = store
	c.allocator = allocator

	return c
}

func newClusterRuntime(cluster metapb.Cluster) *ClusterInfo {
	return &ClusterInfo{
		Meta: cluster,
	}
}

// ClusterInfo The cluster info
type ClusterInfo struct {
	Meta metapb.Cluster `json:"meta"`
}

type cache struct {
	sync.RWMutex

	clusterID uint64
	cluster   *ClusterInfo
	sc        *storeCache
	cc        *cellCache
	store     Store
	allocator *idAllocator
}

func (c *cache) getStoreCache() *storeCache {
	return c.sc
}

func (c *cache) getCellCache() *cellCache {
	return c.cc
}

func (c *cache) allocPeer(storeID uint64) (metapb.Peer, error) {
	peerID, err := c.allocator.newID()
	if err != nil {
		return metapb.Peer{}, errors.Wrap(err, "")
	}

	peer := metapb.Peer{
		ID:      peerID,
		StoreID: storeID,
	}
	return peer, nil
}

func (c *cache) handleCellHeartbeat(source *CellInfo) error {
	current := c.getCellCache().getCell(source.Meta.ID)

	// add new cell
	if nil == current {
		return c.doSaveCellInfo(source)
	}

	// update cell
	currentEpoch := current.Meta.Epoch
	sourceEpoch := source.Meta.Epoch

	// cell meta is stale, return an error.
	if sourceEpoch.CellVer < currentEpoch.CellVer ||
		sourceEpoch.ConfVer < currentEpoch.ConfVer {
		log.Warnf("cell-heartbeat[%d]: cell is stale, current<%d,%d> source<%d,%d>",
			source.Meta.ID,
			currentEpoch.CellVer,
			currentEpoch.ConfVer,
			sourceEpoch.CellVer,
			sourceEpoch.ConfVer)
		return errStaleCell
	}

	// cell meta is updated, update kv and cache.
	if sourceEpoch.CellVer > currentEpoch.CellVer ||
		sourceEpoch.ConfVer > currentEpoch.ConfVer {
		log.Infof("cell-heartbeat[%d]: cell version updated, cellVer=<%d->%d> confVer=<%d->%d>",
			source.Meta.ID,
			currentEpoch.CellVer,
			sourceEpoch.CellVer,
			currentEpoch.ConfVer,
			sourceEpoch.ConfVer)
		return c.doSaveCellInfo(source)
	}

	if current.LeaderPeer != nil && current.LeaderPeer.ID != source.LeaderPeer.ID {
		log.Infof("cell-heartbeat[%d]: update cell leader, from=<%v> to=<%+v>",
			current.getID(),
			current,
			source)
	}

	// cell meta is the same, update cache only.
	c.getCellCache().addOrUpdate(source)
	return nil
}

func (c *cache) doSaveCellInfo(source *CellInfo) error {
	err := c.store.SetCellMeta(c.clusterID, source.Meta)
	if err != nil {
		return err
	}

	c.getCellCache().addOrUpdate(source)
	return nil
}

func randCell(cells map[uint64]*CellInfo) *CellInfo {
	for _, cell := range cells {
		if cell.LeaderPeer == nil {
			log.Fatalf("rand cell without leader: cell=<%+v>", cell)
		}

		if len(cell.DownPeers) > 0 {
			continue
		}

		if len(cell.PendingPeers) > 0 {
			continue
		}

		return cell.clone()
	}

	return nil
}
