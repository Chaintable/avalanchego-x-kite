// Copyright (C) 2019-2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txdenylist

import (
	"github.com/ava-labs/libevm/common"

	"github.com/ava-labs/avalanchego/graft/subnet-evm/precompile/allowlist"
	"github.com/ava-labs/avalanchego/graft/subnet-evm/precompile/contract"
)

// Singleton StatefulPrecompiledContract for W/R access to the tx deny list.
var TxDenyListPrecompile contract.StatefulPrecompiledContract = allowlist.CreateAllowListPrecompile(ContractAddress)

// GetTxDenyListStatus returns the role of [address] for the tx deny list.
func GetTxDenyListStatus(stateDB contract.StateReader, address common.Address) allowlist.Role {
	return allowlist.GetAllowListStatus(stateDB, ContractAddress, address)
}

// IsDenied returns true if [address] is on the deny list (has EnabledRole).
// Admins and Managers are NOT denied — they must be able to transact to manage the list.
func IsDenied(stateDB contract.StateReader, address common.Address) bool {
	return GetTxDenyListStatus(stateDB, address) == allowlist.EnabledRole
}

// SetTxDenyListStatus sets the permissions of [address] to [role] for the
// tx deny list.
// assumes [role] has already been verified as valid.
func SetTxDenyListStatus(stateDB contract.StateDB, address common.Address, role allowlist.Role) {
	allowlist.SetAllowListRole(stateDB, ContractAddress, address, role)
}
