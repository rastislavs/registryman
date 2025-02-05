/*
   Copyright 2021 The Kubermatic Kubernetes Platform contributors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package registry

import (
	"testing"
)

type testRepCap struct {
	push bool
	pull bool
}

func (trp testRepCap) CanPull() bool {
	return trp.pull
}

func (trp testRepCap) CanPush() bool {
	return trp.push
}
func TestComputeReplicationRule(t *testing.T) {
	locNocap := registryCapabilities{false, testRepCap{false, false}}
	locPush := registryCapabilities{false, testRepCap{true, false}}
	locPull := registryCapabilities{false, testRepCap{false, true}}
	locPushPull := registryCapabilities{false, testRepCap{true, true}}
	globNocap := registryCapabilities{true, testRepCap{false, false}}
	globPush := registryCapabilities{true, testRepCap{true, false}}
	globPull := registryCapabilities{true, testRepCap{false, true}}
	globPushPull := registryCapabilities{true, testRepCap{true, true}}

	if calculateReplicationRule(globPush, locPush) != pushReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPush, locPull) != pushReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPush, locPushPull) != pushReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPush, locNocap) != pushReplication {
		t.Error("unexpected result")
	}

	if calculateReplicationRule(locPush, globPush) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPull, globPush) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPushPull, globPush) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locNocap, globPush) != noReplication {
		t.Error("unexpected result")
	}

	if calculateReplicationRule(globPull, locPush) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPull, locPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPull, locPushPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPull, locNocap) != noReplication {
		t.Error("unexpected result")
	}

	if calculateReplicationRule(locPush, globPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPull, globPull) != pullReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPushPull, globPull) != pullReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locNocap, globPull) != noReplication {
		t.Error("unexpected result")
	}

	if calculateReplicationRule(globPushPull, locPush) != pushReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPushPull, locPull) != pushReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPushPull, locPushPull) != pushReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globPushPull, locNocap) != pushReplication {
		t.Error("unexpected result")
	}

	if calculateReplicationRule(locPush, globPushPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPull, globPushPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPushPull, globPushPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locNocap, globPushPull) != noReplication {
		t.Error("unexpected result")
	}

	if calculateReplicationRule(globNocap, locPush) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globNocap, locPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globNocap, locPushPull) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(globNocap, locNocap) != noReplication {
		t.Error("unexpected result")
	}

	if calculateReplicationRule(locPush, globNocap) != noReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPull, globNocap) != pullReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locPushPull, globNocap) != pullReplication {
		t.Error("unexpected result")
	}
	if calculateReplicationRule(locNocap, globNocap) != noReplication {
		t.Error("unexpected result")
	}
}
