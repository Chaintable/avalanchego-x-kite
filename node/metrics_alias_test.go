// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package node

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestPipelineMetricAliaser(t *testing.T) {
	require := require.New(t)

	source := prometheus.NewRegistry()

	headBlock := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "vm_eth_chain_head_block"},
		[]string{"chain"},
	)
	headBlock.WithLabelValues("3USa").Set(103542)
	source.MustRegister(headBlock)

	// A metric not in the alias map must be left out of the aliased output.
	other := prometheus.NewGauge(prometheus.GaugeOpts{Name: "vm_eth_other"})
	other.Set(1)
	source.MustRegister(other)

	out := prometheus.NewRegistry()
	out.MustRegister(newPipelineMetricAliaser(source))

	mfs, err := out.Gather()
	require.NoError(err)

	names := make(map[string]float64)
	for _, mf := range mfs {
		require.Len(mf.GetMetric(), 1)
		m := mf.GetMetric()[0]
		names[mf.GetName()] = m.GetGauge().GetValue()
		// chain label is preserved.
		require.Len(m.GetLabel(), 1)
		require.Equal("chain", m.GetLabel()[0].GetName())
		require.Equal("3USa", m.GetLabel()[0].GetValue())
	}

	// vm_eth_chain_head_block re-emitted as bare chain_head_block with same value.
	require.Equal(float64(103542), names["chain_head_block"])
	// Unmapped metric must not appear under any name.
	require.NotContains(names, "vm_eth_other")
	require.NotContains(names, "vm_eth_chain_head_block")
}
