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

package pd

import (
	"time"

	"golang.org/x/net/context"

	"sync"

	"github.com/deepfabric/elasticell/pkg/log"
	"github.com/deepfabric/elasticell/pkg/pb"
	"github.com/deepfabric/elasticell/pkg/pb/pdpb"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	defaultConnectTimeout = 5 * time.Second
)

// Client pd client
type Client struct {
	name string

	mut   sync.RWMutex
	addrs []string

	continuousFailureCount int64
	conn                   *grpc.ClientConn
	pd                     pdpb.PDServiceClient
	lastAddr               string

	seq uint64
}

// NewClient create a pd client use init pd pdAddrs
func NewClient(name string, initAddrs ...string) (*Client, error) {
	c := new(Client)

	log.Debugf("pd-client: initial pds, pds=<%v>",
		initAddrs)

	c.name = name
	c.addrs = append(c.addrs, initAddrs...)
	c.seq = 0

	err := c.resetConn()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// SetName set name of client
func (c *Client) SetName(name string) {
	c.name = name
}

// Close close conn
func (c *Client) Close() error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if nil != c.conn {
		return c.conn.Close()
	}

	return nil
}

//GetLastPD returns last pd server
func (c *Client) GetLastPD() string {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.lastAddr
}

func (c *Client) resetConn() error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.conn != nil {
		c.conn.Close()
	}

	if c.continuousFailureCount > int64(len(c.addrs)) {
		time.Sleep(time.Second * 10)
	}

	var conn *grpc.ClientConn
	var err error
	for _, addr := range c.addrs {
		log.Debugf("pd-client: try to connect to pd, target=<%s>",
			addr)

		conn, err = createConn(addr)
		if err != nil {
			log.Warnf("pd-client: connect to pd server failure, addr=<%s>, errors: %v",
				addr,
				err)
			c.continuousFailureCount++
			continue
		} else {
			c.lastAddr = addr
			c.continuousFailureCount = 0
			break
		}
	}

	if nil == conn {
		return errors.New("pd-client connect to all init pd servers failure")
	}

	c.conn = conn
	c.pd = pdpb.NewPDServiceClient(conn)

	log.Infof("pd-client: connect to pd server succ, addr=<%s>", c.lastAddr)

	return nil
}

func createConn(addr string) (*grpc.ClientConn, error) {
	return grpc.Dial(addr,
		grpc.WithInsecure(),
		grpc.WithTimeout(defaultConnectTimeout),
		grpc.WithBlock())
}

// GetLeader returns current leader
func (c *Client) GetLeader(ctx context.Context, req *pdpb.LeaderReq) (*pdpb.LeaderRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.GetLeader(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.LeaderRsp), nil
}

// AllocID returns a uniq id
func (c *Client) AllocID(ctx context.Context, req *pdpb.AllocIDReq) (*pdpb.AllocIDRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.AllocID(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.AllocIDRsp), nil
}

// GetClusterID returns cluster id
func (c *Client) GetClusterID(ctx context.Context, req *pdpb.GetClusterIDReq) (*pdpb.GetClusterIDRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.GetClusterID(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.GetClusterIDRsp), nil
}

// IsClusterBootstrapped returns cluster is bootstrapped response
func (c *Client) IsClusterBootstrapped(ctx context.Context, req *pdpb.IsClusterBootstrapReq) (*pdpb.IsClusterBootstrapRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.IsClusterBootstrap(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.IsClusterBootstrapRsp), nil
}

// BootstrapCluster returns bootstrap cluster response
func (c *Client) BootstrapCluster(ctx context.Context, req *pdpb.BootstrapClusterReq) (*pdpb.BootstrapClusterRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.BootstrapCluster(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.BootstrapClusterRsp), nil
}

// PutStore returns put store response
func (c *Client) PutStore(ctx context.Context, req *pdpb.PutStoreReq) (*pdpb.PutStoreRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.PutStore(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.PutStoreRsp), nil
}

// GetStore returns get store response
func (c *Client) GetStore(ctx context.Context, req *pdpb.GetStoreReq) (*pdpb.GetStoreRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.GetStore(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.GetStoreRsp), nil
}

// CellHeartbeat returns cell heartbeat response
func (c *Client) CellHeartbeat(ctx context.Context, req *pdpb.CellHeartbeatReq) (*pdpb.CellHeartbeatRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.CellHeartbeat(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.CellHeartbeatRsp), nil
}

// StoreHeartbeat returns store heartbeat response
func (c *Client) StoreHeartbeat(ctx context.Context, req *pdpb.StoreHeartbeatReq) (*pdpb.StoreHeartbeatRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.StoreHeartbeat(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.StoreHeartbeatRsp), nil
}

// AskSplit returns ask split response
func (c *Client) AskSplit(ctx context.Context, req *pdpb.AskSplitReq) (*pdpb.AskSplitRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.AskSplit(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.AskSplitRsp), nil
}

// ReportSplit returns report split response
func (c *Client) ReportSplit(ctx context.Context, req *pdpb.ReportSplitReq) (*pdpb.ReportSplitRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.ReportSplit(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.ReportSplitRsp), nil
}

// GetLastRanges returns lastest key ranges
func (c *Client) GetLastRanges(ctx context.Context, req *pdpb.GetLastRangesReq) (*pdpb.GetLastRangesRsp, error) {
	rsp, err := c.proxyRPC(ctx,
		req,
		func() {
			req.From = c.name
			req.ID = c.seq
		},
		func() (interface{}, error) {
			return c.pd.GetLastRanges(ctx, req, grpc.FailFast(true))
		})
	if err != nil {
		return nil, err
	}

	return rsp.(*pdpb.GetLastRangesRsp), nil
}

func (c *Client) proxyRPC(ctx context.Context, req pb.BaseReq, setFromFun func(), doRPC func() (interface{}, error)) (interface{}, error) {
	c.mut.RLock()

	if req.GetFrom() == "" && req.GetID() == 0 {
		setFromFun()
		c.seq++
	}

	log.Debugf("pd-client: req<%s-%d>, req=<%v>",
		req.GetFrom(),
		req.GetID(),
		req)

	rsp, err := doRPC()
	if err != nil {
		c.mut.RUnlock()
		if needRetry(err) {
			err = c.resetConn()
			if err != nil {
				return nil, err
			}

			return c.proxyRPC(ctx, req, setFromFun, doRPC)
		}

		return nil, err
	}

	c.mut.RUnlock()
	if err == nil {
		log.Debugf("pd-client: rsp<%s-%d>, rsp=<%v>",
			req.GetFrom(),
			req.GetID(),
			rsp)
	}

	return rsp, nil
}

func needRetry(err error) bool {
	code := grpc.Code(err)

	return codes.Unavailable == code ||
		codes.FailedPrecondition == code
}
