# 上游同步遗留问题

- [ ] Kiro 账号未进入后台主动 token 刷新候选。
  - 现象：`ListOAuthRefreshCandidatePage` 仅选择 `oauth` 与 `setup-token`，不会选择 `type=kiro`；Kiro 刷新执行器虽已注册，但主动刷新周期无法把 Kiro 账号送达执行器。
  - 归因：在同步前基线 `579166b874c6672f5429c68cf217c8dab998669d` 中已存在相同筛选限制，不是 v0.1.156 同步引入的回归。
  - 本次决定：不混入上游同步发布修复；后续应作为独立 feature 明确 Kiro 主动刷新需求、候选类型模型与回归用例。
