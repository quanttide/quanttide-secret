# CHANGELOG

所有显著变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)。

版本遵循语义化版本规范：0.0.x（探索期）→ 0.x.y（验证期）→ x.y.z（正式期）

---

## [Unreleased]

### 新增

- 注册子模块：`apps/qtcloud-secret`、`packages/quanttide-secret-toolkit`、`examples/default`

## [0.1.0-alpha.1] - 2026-08-16

### 新增

- 文档：`docs/dev-guide/`（产品定位、存储选型、传输架构、数据模型、零知识安全设计）+ `docs/user-guide/`（备份与恢复指南：Emergency Kit / 受托恢复）
- 部署 IaC：`manifests/terraform`（OSS 数据桶：版本控制 + SSE-OSS + 生命周期；FC 3.0 应用服务，纯 OSS 方案）
- 服务端：`src/provider`（Go）——JWT 验签（与 qtcloud-auth 共享 JWT_SECRET）、代理 OSS 读写、导出备份 API（`GET /export`）
- CI：`.github/workflows/deploy-provider.yml`（tag `provider/*` 触发：镜像双通道发布 + terraform apply）

### 修复

- Dockerfile 基础镜像对齐 go.mod 工具链要求（go ≥ 1.25），支持 GOPROXY 构建参数覆盖
- JWT 认证对齐 qtcloud-auth：HS256 共享密钥（含 fallback 默认值对齐），修复部署后 FC 环境变量缺失导致的启动失败

## [0.1.0] - 2026-08-16

### 新增

- 初始化密码管理领域仓库
