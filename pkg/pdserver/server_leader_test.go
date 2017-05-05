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
	. "github.com/pingcap/check"
)

func (s *testServerSuite) getLeaderCount() int {
	leaderCount := 0

	for _, svr := range s.servers {
		if svr.IsLeader() {
			leaderCount++
		}
	}

	return leaderCount
}

func (s *testServerSuite) TestServerLeaderCount(c *C) {
	s.restartMultiPDServer(c, 3)

	leaderCount := s.getLeaderCount()
	c.Assert(leaderCount, Equals, 1)

	// for index := 0; index < 3; index++ {
	// 	leaderCount = s.getLeaderCount()
	// 	if leaderCount > 0 {
	// 		break
	// 	}

	// 	time.Sleep(time.Second)
	// }

	// c.Assert(leaderCount, Equals, 1)
}
