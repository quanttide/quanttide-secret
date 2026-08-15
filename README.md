# quanttide-secret

量潮密码管理

## 概述

量潮密码管理（quanttide-secret）是量潮知识管理体系中的**密码管理**领域，涵盖凭证、密钥与敏感信息的全生命周期管理。

## 领域边界

- 凭证管理：口令、API Key、Token 等凭证的登记、轮换与回收
- 密钥管理：签名密钥、加密密钥、应用密钥的生成、存储与分发
- 敏感信息：敏感数据的分类、脱敏、访问控制与审计
- 工具与规范：Vault 等密钥管理服务的接入规范

## 子模块

| 路径 | 说明 |
|------|------|
| `apps/qtcloud-secret` | QtCloud 密码管理应用 (git submodule) |
| `packages/quanttide-secret-toolkit` | 密码管理工具集 (git submodule) |
| `examples/default` | 密码管理实验室 (git submodule → quanttide-laboratory-of-secret-management) |

## 许可

[CC BY 4.0](LICENSE)
