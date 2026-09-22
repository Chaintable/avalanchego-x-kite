# Kite writer v1.15.0 合仓适配

基线：`gokite-ai/avalanchego v1.15.0-kite`（`b25c9fc36`）。该 tag 自报
`v1.15.1`，host 和 plugin 的 RPCChainVM 协议均为 46。

## 代码迁移

| 原实现 | 当前实现 |
|---|---|
| subnet-evm-kite/main 的 core/eth/plugin writer 扩展 | 本仓 graft/subnet-evm 对应目录 |
| plugin 内 vendored GaugeInfo gatherer | 同仓 vms/evm/metrics/prometheus（保留 GaugeInfo 修复） |
| Kite TxDenyList precompile、执行与 txpool 拒绝逻辑 | graft/subnet-evm 对应实现，仅适配包路径和已有工具函数 |
| avalanchego-x-kite/debank 的指标别名、前缀及 AWS 环境传递 | 原位置保留；GaugeInfo 指针工具改用 upstream utils.PointerTo |

libevm 使用已有 `Chaintable/libevm`
`v1.13.15-0.20260903154605-2eaf73af626c-debank-1`，保留 StateDB OnLog/OnCommit、
StateDiff、receipt EffectiveGasPrice 和 GaugeInfo 注册扩展。
pipeline 保持生产版本 `v0.0.64-libevm-ct.3`。

镜像仍由 `Chaintable/subnet-evm-kite` 发布；Dockerfile 从本仓一个固定 commit
构建 host 与 plugin，保留原二进制路径、Kite VMID 和 vm-trace-config。

## 执行与产物语义

- live insert 与恢复时的 reprocess 均设置 block context，交易执行前后采集 tracer，
  状态提交仍使用 libevm 的 OnCommit；非 pipeline tracer 与普通 RPC replay 不初始化 writer。
- libevm 在 root 未变时不调用 OnCommit，因此在 commitWithSnap 成功后补一次空状态回调。
  backup 和 leader 都可生成 header/blockfile/validation，只有 leader 进入 Kafka 通知分支。
  此处修复了旧版仅在 leader 分支补空状态产物的问题；不改变 EVM 执行和状态根。
- invalid transaction 没有 receipt，先返回原始 consensus error，不调用 receipt 转换；
  revert 交易仍有失败 receipt，按既有 pipeline 语义采集。
- 新块的共识规则来自固定 upstream tag。TxDenyList 仅在链配置显式启用后生效；
  本次现场官方与生产主网配置均未启用它，保留该扩展以避免静默丢失既有能力。
- C-Chain SAE 的 RPC 删除和执行语义变化不能直接用于推断 Kite Subnet-EVM 行为。

## 本地验证

2026-09-22，macOS arm64 / Go 1.26.4：

- `go build ./... ./graft/coreth/... ./graft/evm/... ./graft/subnet-evm/...` 通过。
- host/plugin 二进制均可执行 `--version`，协议号一致为 46。
- core、TxDenyList、plugin config、Prometheus gatherer 四包完整单测：
  472 个测试/子测试通过，15 个 upstream 已有 skip，无失败。skip 包括 12 个
  Firewood+snapshot 不支持组合，以及 offline pruning/chain indexer/skip indexing
  既有暂停测试；本次未新增 skip。
- node 的 BareAliasName/PipelineMetricAliaser 定向测试通过。
- 新增回归验证空状态块在无 leader/pusher 时产生一次回调、reprocess 保留回调、
  invalid nonce 在启用 pipeline tracer 时返回原始错误而不解引用空 receipt。

尚未验证 Linux 镜像构建、主网追块、MinIO 实际投递与产物对账；本报告不构成生产发布验收。
生产 pipeline 的旧 S3 retry quota panic 风险仍存在，不在此次 upstream 适配内修复。
