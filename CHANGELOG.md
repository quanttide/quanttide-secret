# CHANGELOG

所有显著变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)。

版本遵循语义化版本规范：0.0.x（探索期）→ 0.x.y（验证期）→ x.y.z（正式期）

---

## [Unreleased]

### 变更

- 领域更名：密码管理 → 机密管理；范围扩大为密码、证件等所有需要同一套安全架构处理的对象（不同对象形态使用不同数据模型）

### 新增

- 注册子模块：`apps/qtcloud-secret`、`packages/quanttide-secret-toolkit`、`examples/default`
- 应用代码迁移至 `apps/qtcloud-secret` 仓库（服务端 / IaC / 文档，见该仓库 CHANGELOG）

## [0.1.0] - 2026-08-16

### 新增

- 初始化密码管理领域仓库
