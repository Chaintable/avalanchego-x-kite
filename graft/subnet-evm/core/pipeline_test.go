package core

import (
	"math/big"
	"testing"

	"github.com/Chaintable/pipeline/tracer"
	"github.com/ava-labs/avalanchego/graft/subnet-evm/consensus/dummy"
	"github.com/ava-labs/avalanchego/graft/subnet-evm/core/tracing"
	"github.com/ava-labs/avalanchego/graft/subnet-evm/params"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/rawdb"
	"github.com/ava-labs/libevm/core/types"
	"github.com/ava-labs/libevm/core/vm"
	"github.com/ava-labs/libevm/crypto"
	"github.com/stretchr/testify/require"
)

// Empty-state blocks must be emitted even without a leader or Kafka pusher.
// Replaying an accepted block must execute the same block/commit hooks.
func TestPipelineEmptyStateCommit(t *testing.T) {
	config := params.Copy(params.TestChainConfig)
	genesis := &Genesis{
		Config:   &config,
		GasLimit: params.GetExtra(&config).FeeConfig.GasLimit.Uint64(),
	}
	_, blocks, _, err := GenerateChainWithGenesis(genesis, dummy.NewFaker(), 2, 10, func(_ int, block *BlockGen) {
		block.SetCoinbase(common.HexToAddress("0x0100000000000000000000000000000000000000"))
	})
	require.NoError(t, err)
	bc, err := NewBlockChain(rawdb.NewMemoryDatabase(), archiveConfig, genesis, dummy.NewFaker(), vm.Config{}, common.Hash{}, false)
	require.NoError(t, err)
	t.Cleanup(bc.Stop)
	var starts, commits []uint64
	var current uint64
	bc.hooks = &tracing.Hooks{
		OnBlockStart: func(block *types.Block) {
			current = block.NumberU64()
			starts = append(starts, current)
		},
		OnCommit: func(origin, root common.Hash, _ map[common.Hash]struct{}, _ map[common.Hash][]byte, _ map[common.Address][]byte, _ map[common.Hash]map[common.Hash][]byte, _ map[common.Address]map[common.Hash][]byte, _ map[common.Hash][]byte) {
			require.Equal(t, origin, root)
			commits = append(commits, current)
		},
	}
	_, err = bc.InsertChain(blocks)
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2}, starts)
	require.Equal(t, starts, commits)
	_, err = bc.reprocessBlock(bc.Genesis(), blocks[0])
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2, 1}, starts)
	require.Equal(t, starts, commits)
}

// Invalid consensus transactions return their original error; they have no
// receipt and must not reach the pipeline's receipt conversion.
func TestPipelineInvalidTransaction(t *testing.T) {
	config := params.Copy(params.TestChainConfig)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	from := crypto.PubkeyToAddress(key.PublicKey)
	genesis := &Genesis{
		Config:   &config,
		GasLimit: params.GetExtra(&config).FeeConfig.GasLimit.Uint64(),
		Alloc:    types.GenesisAlloc{from: {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(20), nil)}},
	}
	bc, err := NewBlockChain(rawdb.NewMemoryDatabase(), archiveConfig, genesis, dummy.NewFullFaker(), vm.Config{}, common.Hash{}, false)
	require.NoError(t, err)
	t.Cleanup(bc.Stop)
	header := &types.Header{
		Number: big.NewInt(1), ParentHash: bc.Genesis().Hash(),
		GasLimit: genesis.GasLimit, BaseFee: big.NewInt(25_000_000_000),
	}
	tx, err := types.SignTx(types.NewTransaction(100, common.Address{1}, big.NewInt(0), 21000, header.BaseFee, nil), types.LatestSigner(&config), key)
	require.NoError(t, err)
	block := types.NewBlockWithHeader(header).WithBody(types.Body{Transactions: []*types.Transaction{tx}})
	state, err := bc.State()
	require.NoError(t, err)
	pipelineTracer := &tracer.PipelineTracer{}
	pipelineTracer.OnBlockStart(block)
	defer func() { tracer.BlockCtx = nil }()
	_, _, _, err = bc.processor.Process(block, bc.Genesis().Header(), state, vm.Config{Tracer: pipelineTracer})
	require.ErrorContains(t, err, "nonce too high")
	require.Empty(t, tracer.BlockCtx.BlockFile.Txs)
}
