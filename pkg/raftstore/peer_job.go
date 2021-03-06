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

package raftstore

import (
	"context"
	"fmt"

	"github.com/deepfabric/elasticell/pkg/log"
	"github.com/deepfabric/elasticell/pkg/pb/metapb"
	"github.com/deepfabric/elasticell/pkg/pb/mraft"
	"github.com/deepfabric/elasticell/pkg/pb/pdpb"
	"github.com/deepfabric/elasticell/pkg/util"
	"github.com/deepfabric/etcd/raft/raftpb"
)

func (pr *PeerReplicate) startApplyingSnapJob() {
	pr.ps.applySnapJobLock.Lock()
	job, err := pr.store.addApplyJob(pr.cellID, pr.doApplyingSnapshotJob)
	if err != nil {
		log.Fatalf("raftstore[cell-%d]: add apply snapshot task fail, errors:\n %+v",
			pr.cellID,
			err)
	}

	pr.ps.applySnapJob = job
	pr.ps.applySnapJobLock.Unlock()
}

func (ps *peerStorage) startDestroyDataJob(cellID uint64, start, end []byte) error {
	_, err := ps.store.addApplyJob(cellID, func() error {
		return ps.doDestroyDataJob(cellID, start, end)
	})

	return err
}

func (pr *PeerReplicate) startRegistrationJob() {
	delegate := &applyDelegate{
		store:            pr.store,
		ps:               pr.ps,
		peerID:           pr.peer.ID,
		cell:             pr.ps.getCell(),
		term:             pr.getCurrentTerm(),
		applyState:       *pr.ps.getApplyState(),
		appliedIndexTerm: pr.ps.getAppliedIndexTerm(),
	}

	_, err := pr.store.addApplyJob(pr.cellID, func() error {
		return pr.doRegistrationJob(delegate)
	})

	if err != nil {
		log.Fatalf("raftstore[cell-%d]: add registration job failed, errors:\n %+v",
			pr.ps.getCell().ID,
			err)
	}
}

func (pr *PeerReplicate) startApplyCommittedEntriesJob(cellID uint64, term uint64, commitedEntries []raftpb.Entry) error {
	_, err := pr.store.addApplyJob(pr.cellID, func() error {
		return pr.doApplyCommittedEntries(cellID, term, commitedEntries)
	})
	return err
}

func (pr *PeerReplicate) startRaftLogGCJob(cellID, startIndex, endIndex uint64) error {
	_, err := pr.store.addRaftLogGCJob(func() error {
		return pr.doRaftLogGC(cellID, startIndex, endIndex)
	})

	return err
}

func (s *Store) startDestroyJob(cellID uint64) error {
	_, err := s.addApplyJob(cellID, func() error {
		return s.doDestroy(cellID)
	})

	return err
}

func (pr *PeerReplicate) startProposeJob(meta *proposalMeta, isConfChange bool) error {
	_, err := pr.store.addApplyJob(pr.cellID, func() error {
		return pr.doPropose(meta, isConfChange)
	})

	return err
}

func (pr *PeerReplicate) startSplitCheckJob() error {
	cell := pr.getCell()
	epoch := cell.Epoch
	startKey := encStartKey(&cell)
	endKey := encEndKey(&cell)

	_, err := pr.store.addSplitJob(func() error {
		return pr.doSplitCheck(epoch, startKey, endKey)
	})

	return err
}

func (pr *PeerReplicate) startAskSplitJob(cell metapb.Cell, peer metapb.Peer, splitKey []byte) error {
	_, err := pr.store.addSplitJob(func() error {
		return pr.doAskSplit(cell, peer, splitKey)
	})

	return err
}

func (s *Store) startReportSpltJob(left metapb.Cell, right metapb.Cell) error {
	_, err := s.addPDJob(func() error {
		_, err := s.pdClient.ReportSplit(context.TODO(), &pdpb.ReportSplitReq{
			Left:  left,
			Right: right,
		})

		return err
	})

	return err
}

func (ps *peerStorage) cancelApplyingSnapJob() bool {
	ps.applySnapJobLock.RLock()
	if ps.applySnapJob == nil {
		ps.applySnapJobLock.RUnlock()
		return true
	}

	ps.applySnapJob.Cancel()

	if ps.applySnapJob.IsCancelled() {
		ps.applySnapJobLock.RUnlock()
		return true
	}

	succ := !ps.isApplyingSnap()
	ps.applySnapJobLock.RUnlock()
	return succ
}

func (ps *peerStorage) resetApplyingSnapJob() {
	ps.applySnapJobLock.Lock()
	ps.applySnapJob = nil
	ps.applySnapJobLock.Unlock()
}

func (ps *peerStorage) resetGenSnapJob() {
	ps.genSnapJob = nil
	ps.snapTriedCnt = 0
}

func (ps *peerStorage) doDestroyDataJob(cellID uint64, startKey, endKey []byte) error {
	log.Infof("raftstore[cell-%d]: deleting data, start=<%v>, end=<%v>",
		cellID,
		startKey,
		endKey)

	err := ps.deleteAllInRange(startKey, endKey, nil)
	if err != nil {
		log.Errorf("raftstore[cell-%d]: failed to delete data, start=<%v> end=<%v> errors:\n %+v",
			cellID,
			startKey,
			endKey,
			err)
	}

	return err
}

func (pr *PeerReplicate) doApplyingSnapshotJob() error {
	log.Infof("raftstore[cell-%d]: begin apply snap data", pr.cellID)
	localState, err := pr.ps.loadCellLocalState(pr.ps.applySnapJob)
	if err != nil {
		return err
	}

	err = pr.ps.deleteAllInRange(encStartKey(&localState.Cell), encEndKey(&localState.Cell), pr.ps.applySnapJob)
	if err != nil {
		log.Errorf("raftstore[cell-%d]: apply snap delete range data failed, errors:\n %+v",
			pr.cellID,
			err)
		return err
	}

	err = pr.ps.applySnapshot(pr.ps.applySnapJob)
	if err != nil {
		log.Errorf("raftstore[cell-%d]: apply snap snapshot failed, errors:\n %+v",
			pr.cellID,
			err)
		return err
	}

	err = pr.ps.updatePeerState(pr.ps.getCell(), mraft.Normal, nil)
	if err != nil {
		log.Errorf("raftstore[cell-%d]: apply snap update peer state failed, errors:\n %+v",
			pr.cellID,
			err)
		return err
	}

	log.Infof("raftstore[cell-%d]: apply snap complete", pr.cellID)
	return nil
}

func (ps *peerStorage) doGenerateSnapshotJob() error {
	if ps.genSnapJob == nil {
		log.Fatalf("raftstore[cell-%d]: generating snapshot job chan is nil", ps.getCell().ID)
	}

	applyState, err := ps.loadApplyState()
	if err != nil {
		log.Errorf("raftstore[cell-%d]: load snapshot failure, errors:\n %+v",
			ps.getCell().ID,
			err)
		return nil
	} else if nil == applyState {
		log.Errorf("raftstore[cell-%d]: could not load snapshot", ps.getCell().ID)
		return nil
	}

	var term uint64
	if applyState.AppliedIndex == applyState.TruncatedState.Index {
		term = applyState.TruncatedState.Term
	} else {
		entry, err := ps.loadLogEntry(applyState.AppliedIndex)
		if err != nil {
			return nil
		}

		term = entry.Term
	}

	state, err := ps.loadCellLocalState(nil)
	if err != nil {
		return nil
	}

	if state.State != mraft.Normal {
		log.Errorf("raftstore[cell-%d]: snap seems stale, skip", ps.getCell().ID)
		return nil
	}

	key := mraft.SnapKey{
		CellID: ps.getCell().ID,
		Term:   term,
		Index:  applyState.AppliedIndex,
	}

	snapshot := raftpb.Snapshot{}
	snapshot.Metadata.Term = key.Term
	snapshot.Metadata.Index = key.Index

	confState := raftpb.ConfState{}
	for _, peer := range ps.getCell().Peers {
		confState.Nodes = append(confState.Nodes, peer.ID)
	}
	snapshot.Metadata.ConfState = confState

	snapData := &mraft.RaftSnapshotData{}
	snapData.Cell = state.Cell
	snapData.Key = key

	if ps.store.snapshotManager.Register(&key, creating) {
		defer ps.store.snapshotManager.Deregister(&key, creating)

		err = ps.store.snapshotManager.Create(snapData)
		if err != nil {
			log.Errorf("raftstore[cell-%d]: create snapshot file failure, errors:\n %+v",
				ps.getCell().ID,
				err)
			return nil
		}
	}

	snapshot.Data = util.MustMarshal(snapData)

	log.Infof("raftstore[cell-%d]: snapshot complete", ps.getCell().ID)
	ps.genSnapJob.SetResult(snapshot)

	return nil
}

func (pr *PeerReplicate) doRegistrationJob(delegate *applyDelegate) error {
	old := pr.store.delegates.put(delegate.cell.ID, delegate)
	if old != nil {
		if old.peerID != delegate.peerID {
			log.Fatalf("raftstore[cell-%d]: delegate peer id not match, old=<%d> curr=<%d>",
				pr.cellID,
				old.peerID,
				delegate.peerID)
		}

		old.term = delegate.term
		old.clearAllCommandsAsStale()
	}

	return nil
}

func (s *Store) doDestroy(cellID uint64) error {
	d := s.delegates.delete(cellID)
	if d != nil {
		d.destroy()
		// TODO: think send notify, then liner process this and other apply result
		s.destroyPeer(cellID, metapb.Peer{ID: d.peerID, StoreID: s.GetID()}, false)
	}

	return nil
}

func (pr *PeerReplicate) doApplyCommittedEntries(cellID uint64, term uint64, commitedEntries []raftpb.Entry) error {
	delegate := pr.store.delegates.get(cellID)
	if nil == delegate {
		return fmt.Errorf("raftstore[cell-%d]: missing delegate", pr.cellID)
	}

	delegate.term = term
	delegate.applyCommittedEntries(commitedEntries)

	if delegate.isPendingRemove() {
		delegate.destroy()
		pr.store.delegates.delete(delegate.cell.ID)
	}

	return nil
}

func (pr *PeerReplicate) doRaftLogGC(cellID, startIndex, endIndex uint64) error {
	firstIndex := startIndex

	if firstIndex == 0 {
		startKey := getRaftLogKey(cellID, 0)
		firstIndex = endIndex
		key, _, err := pr.store.engine.GetEngine().Seek(startKey)
		if err != nil {
			return err
		}

		if key != nil {
			firstIndex, err = getRaftLogIndex(key)
			if err != nil {
				return err
			}
		}
	}

	if firstIndex >= endIndex {
		log.Infof("raftstore-compact[cell-%d]: no need to gc raft log",
			cellID)
		return nil
	}

	wb := pr.store.engine.NewWriteBatch()
	for index := firstIndex; index < endIndex; index++ {
		key := getRaftLogKey(cellID, index)
		err := wb.Delete(key)
		if err != nil {
			return err
		}
	}

	err := pr.store.engine.Write(wb)
	if err != nil {
		log.Infof("raftstore-compact[cell-%d]: raft log gc complete, entriesCount=<%d>",
			cellID,
			(endIndex - startIndex))
	}

	return err
}

func (ps *peerStorage) isApplyingSnap() bool {
	return ps.applySnapJob != nil && ps.applySnapJob.IsNotComplete()
}

func (ps *peerStorage) isGeneratingSnap() bool {
	return ps.genSnapJob != nil && ps.genSnapJob.IsNotComplete()
}

func (ps *peerStorage) isGenSnapJobComplete() bool {
	return ps.genSnapJob != nil && ps.genSnapJob.IsComplete()
}
