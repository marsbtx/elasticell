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
	"github.com/deepfabric/elasticell/pkg/log"
	"github.com/deepfabric/elasticell/pkg/pdapi"
)

// ListStore returns all store info
func (s *Server) ListStore() ([]*pdapi.StoreInfo, error) {
	cluster := s.GetCellCluster()
	if nil == cluster {
		return nil, errNotBootstrapped
	}

	return toAPIStoreSlice(cluster.cache.getStoreCache().getStores()), nil
}

// GetStore return the store with the id
func (s *Server) GetStore(id uint64) (*pdapi.StoreInfo, error) {
	cluster := s.GetCellCluster()
	if nil == cluster {
		return nil, errNotBootstrapped
	}

	c := cluster.cache.getStoreCache()
	return toAPIStore(c.getStore(id)), nil
}

// DeleteStore remove the store from cluster
// all cells on this store will move to another stores
func (s *Server) DeleteStore(id uint64, force bool) error {
	cluster := s.GetCellCluster()
	if nil == cluster {
		return errNotBootstrapped
	}

	var store *StoreInfo
	var err error
	if force {
		store, err = cluster.cache.getStoreCache().setStoreTombstone(id, force)
	} else {
		store, err = cluster.cache.getStoreCache().setStoreOffline(id)
	}

	if err != nil {
		return err
	}

	if nil != store {
		err = s.store.SetStoreMeta(id, store.Meta)
		if err != nil {
			return err
		}

		log.Infof("[store-%d] store has been %s, address=<%s>",
			store.Meta.ID,
			store.Meta.State.String(),
			store.Meta.Address)
		cluster.cache.getStoreCache().updateStoreInfo(store)
	}

	return nil
}

func toAPIStore(store *StoreInfo) *pdapi.StoreInfo {
	return &pdapi.StoreInfo{
		Meta: store.Meta,
		Status: &pdapi.StoreStatus{
			Stats:           store.Status.Stats,
			LeaderCount:     store.Status.LeaderCount,
			LastHeartbeatTS: store.Status.LastHeartbeatTS,
		},
	}
}

func toAPIStoreSlice(stores []*StoreInfo) []*pdapi.StoreInfo {
	values := make([]*pdapi.StoreInfo, len(stores))

	for idx, store := range stores {
		values[idx] = toAPIStore(store)
	}

	return values
}
