// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package node

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	dto "github.com/prometheus/client_model/go"
)

func TestBareAliasName(t *testing.T) {
	for name, want := range map[string]string{
		"vm_eth_chain_head_block":    "chain_head_block",
		"vm_eth_chain_execution":     "chain_execution",
		"vm_eth_pipeline_block_num":  "pipeline_block_num",
		"vm_eth_pipeline_node_info":  "pipeline_node_info",
		"vm_eth_eth_protocols":       "", // not chain_/pipeline_
		"vm_eth_txpool_pending":      "", // not chain_/pipeline_
		"chain_cache_len":            "", // bare avalanchego metric, no prefix
		"avalanche_network_peers":    "", // no prefix
		"vm_eth_chainless":           "", // chain_ prefix requires the underscore
		"vm_eth_pipeline_block_time": "pipeline_block_time",
	} {
		bare, ok := bareAliasName(name)
		if want == "" {
			require.False(t, ok, "name %q must not be aliased", name)
		} else {
			require.True(t, ok, "name %q must be aliased", name)
			require.Equal(t, want, bare)
		}
	}
}

func TestPipelineMetricAliaser(t *testing.T) {
	require := require.New(t)

	source := prometheus.NewRegistry()

	headBlock := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "vm_eth_chain_head_block"},
		[]string{"chain"},
	)
	headBlock.WithLabelValues("3USa").Set(103542)
	source.MustRegister(headBlock)

	// Info-style gauge as produced by the gatherer for pipeline/node_info
	// (GaugeInfo): constant 1 with the data in the labels.
	nodeInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "vm_eth_pipeline_node_info"},
		[]string{"chain", "chain_id", "role"},
	)
	nodeInfo.WithLabelValues("3USa", "2366", "writer").Set(1)
	source.MustRegister(nodeInfo)

	// Timer-backed metrics arrive as summaries (quantiles + count + sum).
	blockPush := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "vm_eth_pipeline_block_push",
			Objectives: map[float64]float64{0.5: 0.05},
		},
		[]string{"chain"},
	)
	blockPush.WithLabelValues("3USa").Observe(2)
	blockPush.WithLabelValues("3USa").Observe(4)
	source.MustRegister(blockPush)

	// A vm_eth_ metric outside chain_/pipeline_ must be left out.
	other := prometheus.NewGauge(prometheus.GaugeOpts{Name: "vm_eth_other"})
	other.Set(1)
	source.MustRegister(other)

	out := prometheus.NewRegistry()
	out.MustRegister(newPipelineMetricAliaser(source))

	mfs, err := out.Gather()
	require.NoError(err)

	families := make(map[string]*dto.MetricFamily)
	for _, mf := range mfs {
		families[mf.GetName()] = mf
	}

	// vm_eth_chain_head_block re-emitted as bare chain_head_block with the
	// chain label preserved.
	hb := families["chain_head_block"]
	require.NotNil(hb)
	require.Len(hb.GetMetric(), 1)
	require.Equal(float64(103542), hb.GetMetric()[0].GetGauge().GetValue())
	require.Equal("chain", hb.GetMetric()[0].GetLabel()[0].GetName())
	require.Equal("3USa", hb.GetMetric()[0].GetLabel()[0].GetValue())

	// vm_eth_pipeline_node_info re-emitted bare with all labels preserved.
	ni := families["pipeline_node_info"]
	require.NotNil(ni)
	require.Len(ni.GetMetric(), 1)
	require.Equal(float64(1), ni.GetMetric()[0].GetGauge().GetValue())
	labels := make(map[string]string)
	for _, lp := range ni.GetMetric()[0].GetLabel() {
		labels[lp.GetName()] = lp.GetValue()
	}
	require.Equal(map[string]string{
		"chain":    "3USa",
		"chain_id": "2366",
		"role":     "writer",
	}, labels)

	// vm_eth_pipeline_block_push re-emitted as a bare summary with count, sum
	// and quantiles preserved.
	bp := families["pipeline_block_push"]
	require.NotNil(bp)
	require.Len(bp.GetMetric(), 1)
	s := bp.GetMetric()[0].GetSummary()
	require.NotNil(s)
	require.Equal(uint64(2), s.GetSampleCount())
	require.Equal(float64(6), s.GetSampleSum())
	require.Len(s.GetQuantile(), 1)
	require.Equal(float64(2), s.GetQuantile()[0].GetValue())

	// Non-matching families must not appear under any name.
	require.NotContains(families, "vm_eth_other")
	require.NotContains(families, "other")
	require.NotContains(families, "vm_eth_chain_head_block")
}
