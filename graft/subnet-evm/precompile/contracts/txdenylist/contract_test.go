// Copyright (C) 2019-2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txdenylist_test

import (
	"github.com/ava-labs/avalanchego/graft/subnet-evm/precompile/contracts/txdenylist"
	"testing"

	"github.com/ava-labs/avalanchego/graft/subnet-evm/precompile/allowlist/allowlisttest"
)

func TestTxDenyListRun(t *testing.T) {
	allowlisttest.RunPrecompileWithAllowListTests(t, txdenylist.Module, nil)
}
