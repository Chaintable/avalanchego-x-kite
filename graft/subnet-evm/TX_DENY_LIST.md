# TxDenyList Precompile

Transaction-level blacklist mechanism for Subnet EVM. Allows chain administrators to block specific addresses from sending transactions, without affecting other users.

## Overview

| Item | Value |
|------|-------|
| Contract Address | `0x0300000000000000000000000000000000000000` |
| Config Key | `txDenyListConfig` |
| Mechanism | Blacklist (default: all addresses allowed; only explicitly denied addresses are blocked) |

### Role Semantics

| Role | Value | Meaning | Can Transact? | Can Manage List? |
|------|-------|---------|---------------|-----------------|
| NoRole | 0 | Not on deny list | Yes | No |
| EnabledRole | 1 | **On deny list (blocked)** | **No** | No |
| AdminRole | 2 | Deny list administrator | Yes | Yes (full control) |
| ManagerRole | 3 | Deny list manager | Yes | Yes (limited: can only add/remove EnabledRole) |

### ABI Functions

The precompile reuses the standard AllowList ABI at the contract address:

| Function | Signature | In DenyList Context |
|----------|-----------|---------------------|
| `setEnabled(address)` | `0x0aaf7043` | Add address to deny list (block) |
| `setNone(address)` | `0x8c6bfb3b` | Remove address from deny list (unblock) |
| `setAdmin(address)` | `0xc6b2d3d4` | Promote to admin |
| `setManager(address)` | `0xd0ebdbe7` | Promote to manager |
| `readAllowList(address)` | `0xeb54dae1` | Query role (0=not denied, 1=denied, 2=admin, 3=manager) |

## Network Upgrade: Activating TxDenyList on a Running Chain

### Prerequisites

- New subnet-evm binary built with the `txdenylist` precompile
- All validator nodes must be upgraded before the activation timestamp

### Method 1: Via `upgrade.json` File

Place the file at `<chain-config-dir>/<blockchainID>/upgrade.json`:

```json
{
  "precompileUpgrades": [
    {
      "txDenyListConfig": {
        "blockTimestamp": 1720000000,
        "adminAddresses": ["0xYourAdminAddress"]
      }
    }
  ]
}
```

- `blockTimestamp`: Unix timestamp (seconds) in the future when the precompile activates
- `adminAddresses`: One or more addresses that can manage the deny list (recommend multi-sig)
- Optionally include `"enabledAddresses"` to pre-populate the deny list at activation time

### Method 2: Via Environment Variable (for EKS/Kubernetes)

Use `AVALANCHEGO_CHAIN_CONFIG_CONTENT` to pass config inline without files.

#### Encoding Structure

```
AVALANCHEGO_CHAIN_CONFIG_CONTENT = base64(
  {
    "<blockchainID>": {
      "Config": "<base64(config.json contents)>",
      "Upgrade": "<base64(upgrade.json contents)>"
    }
  }
)
```

#### Step-by-Step

```bash
# 1. Prepare upgrade.json content
UPGRADE_JSON='{
  "precompileUpgrades": [
    {
      "txDenyListConfig": {
        "blockTimestamp": 1720000000,
        "adminAddresses": ["0xYourAdminAddress"]
      }
    }
  ]
}'

# 2. Prepare config.json content (your existing chain config)
CONFIG_JSON='{
  "log-level": "info",
  "eth-apis": ["eth", "net", "web3"]
}'

# 3. Base64 encode each
UPGRADE_B64=$(echo -n "$UPGRADE_JSON" | base64)
CONFIG_B64=$(echo -n "$CONFIG_JSON" | base64)

# 4. Assemble outer JSON and base64 encode
BLOCKCHAIN_ID="your-blockchain-id-here"
OUTER_JSON="{\"${BLOCKCHAIN_ID}\":{\"Config\":\"${CONFIG_B64}\",\"Upgrade\":\"${UPGRADE_B64}\"}}"
CHAIN_CONFIG_CONTENT=$(echo -n "$OUTER_JSON" | base64)

# 5. Set environment variable
export AVALANCHEGO_CHAIN_CONFIG_CONTENT="$CHAIN_CONFIG_CONTENT"
```

#### Kubernetes Deployment Example

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: avalanche-validator
spec:
  template:
    spec:
      containers:
        - name: avalanchego
          env:
            # Option A: Inline value
            - name: AVALANCHEGO_CHAIN_CONFIG_CONTENT
              value: "eyJhYmNkZWYxMjM0NS4uLi..."

            # Option B: From ConfigMap
            - name: AVALANCHEGO_CHAIN_CONFIG_CONTENT
              valueFrom:
                configMapKeyRef:
                  name: avalanche-chain-config
                  key: chain-config-content

            # Option C: From Secret (if admin keys are sensitive)
            - name: AVALANCHEGO_CHAIN_CONFIG_CONTENT
              valueFrom:
                secretKeyRef:
                  name: avalanche-chain-config
                  key: chain-config-content
```

### Upgrade Procedure

1. **Build new image** — Include the subnet-evm binary with txdenylist precompile
2. **Choose activation timestamp** — Pick a time far enough in the future (e.g., 1 hour) for all nodes to restart
3. **Update configuration** — Set `AVALANCHEGO_CHAIN_CONFIG_CONTENT` with the upgrade content (or deploy `upgrade.json`)
4. **Rolling restart all validators** — All nodes MUST be upgraded before `blockTimestamp`
5. **Verify activation** — After the timestamp passes, the precompile is active

### Important Notes

- If any validator is not upgraded by the activation timestamp, it will fork from the network
- The `AVALANCHEGO_CHAIN_CONFIG_CONTENT` flag takes priority over `--chain-config-dir` file-based config
- Once activated, the upgrade entry cannot be removed from the config (it's part of consensus rules)
- The `blockTimestamp` must be strictly increasing for subsequent upgrades

## Usage: Managing the Deny List

### Block an Address (Add to Deny List)

```bash
cast send 0x0300000000000000000000000000000000000000 \
  "setEnabled(address)" \
  0xHackerAddress \
  --private-key $ADMIN_PRIVATE_KEY \
  --rpc-url $RPC_URL
```

### Unblock an Address (Remove from Deny List)

```bash
cast send 0x0300000000000000000000000000000000000000 \
  "setNone(address)" \
  0xHackerAddress \
  --private-key $ADMIN_PRIVATE_KEY \
  --rpc-url $RPC_URL
```

### Check if an Address is Denied

```bash
cast call 0x0300000000000000000000000000000000000000 \
  "readAllowList(address)(uint256)" \
  0xSuspectAddress \
  --rpc-url $RPC_URL
```

Returns:
- `0` — Not denied (can transact)
- `1` — **Denied (blocked)**
- `2` — Admin
- `3` — Manager

### Promote a Manager

Managers can add/remove addresses from the deny list but cannot promote other admins/managers:

```bash
cast send 0x0300000000000000000000000000000000000000 \
  "setManager(address)" \
  0xSecurityTeamAddress \
  --private-key $ADMIN_PRIVATE_KEY \
  --rpc-url $RPC_URL
```

## Interaction with TxAllowList

TxDenyList and TxAllowList are independent and can coexist:

| TxAllowList | TxDenyList | Result |
|-------------|------------|--------|
| Disabled | Enabled | Open chain + blacklist (most common use case) |
| Enabled | Disabled | Permissioned chain (whitelist only) |
| Enabled | Enabled | Must be on allowlist AND not on denylist |
| Disabled | Disabled | Fully open chain |

## Solidity Integration

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface ITxDenyList {
    function setEnabled(address addr) external;   // deny
    function setNone(address addr) external;      // un-deny
    function setAdmin(address addr) external;
    function setManager(address addr) external;
    function readAllowList(address addr) external view returns (uint256);
}

ITxDenyList constant TX_DENY_LIST = ITxDenyList(0x0300000000000000000000000000000000000000);
```

## Genesis Configuration (New Chain)

For new chains, enable TxDenyList directly in genesis:

```json
{
  "config": {
    "chainId": 99999,
    "txDenyListConfig": {
      "blockTimestamp": 0,
      "adminAddresses": ["0xAdminAddress1", "0xAdminAddress2"],
      "enabledAddresses": ["0xKnownBadActor1"]
    }
  }
}
```
