# qtcloud-secret 部署选型（IaC）

对齐 qtcloud-delib 的部署决策（系统级资源由 quanttide-platform 管理），作为 Terraform 基础设施代码的设计依据。架构设计见 `docs/dev-guide/`（storage.md / transfer.md / model.md）。

## 部署选型

| 维度 | 选型 | 说明 |
|------|------|------|
| 存储 | 阿里云 OSS（版本控制 + SSE-OSS + 生命周期） | 当前阶段（单团队、小数据量）纯 OSS，无 PG/RDS；密文信封对象 `secrets/<id>.json`；版本控制兜底误删误写，SSE-OSS 第二层加密（免费），生命周期清理历史版本防膨胀 |
| 服务计算 | FaaS（函数计算 FC 3.0）+ custom-container | Dockerfile 构建镜像，双通道发布（Docker Hub 对外分发 + 阿里云 ACR 同地域直拉）；服务无需常驻、按调用计费 |
| 数据库 | **当前阶段无**（纯 OSS 方案） | 单团队无 RBAC 关系需求；团队版引入 PG 时密文对象零迁移（见 model.md 扩展位） |
| 网络 | 无 VPC 需求 | FC 经公网 endpoint 访问 OSS（无 RDS 内网互通需求） |
| 密钥管理 | **预留（HashiCorp Vault，待引入）** | 服务端密钥底座（OSS 动态凭证 / JWT 密钥 / DB 口令 / transit），替代 KMS 3.0 实例方案（社区版免费）；当前阶段无服务端密钥托管需求，等团队版/私有化立项时落地（见 security.md 第 9 章） |
| API 网关 | **预留（系统层面统一规划）** | 统一 `api.quanttide.com`，路径按应用名（如 `/qtcloud-secret`）；不在本应用 IaC 范围内 |

## 本 IaC 范围

- **系统级共享**（quanttide 体系统一管理，`quanttide-<env>` 命名）：资源组（`platform.tf` 远程状态引用）
- **应用级**（`qtcloud-secret-<env>` 命名）：
  - OSS 数据桶（版本控制 + SSE-OSS + 生命周期 + 私有 ACL，`oss.tf`）
  - FC 函数与默认角色（最小权限 RAM 策略：仅数据桶 `secrets/` 读写，`fc.tf`）
- **不含** API 网关、域名、DNS（系统层面预留）

## 安全说明

- **双重加密**：客户端 AES-256-GCM（明文不离开客户端）+ OSS SSE-OSS（第二层兜底）
- **服务端加密成本**：SSE-OSS 由 OSS 托管密钥、自动轮换、**免费**。不用 SSE-KMS 的原因：KMS 共享版（按密钥计费、免费额度内免密钥费）已 EOFS/EOS 停服，当前新购 KMS 密钥只能购买专属密钥管理实例（软件实例 2,499 元/月起），当前阶段不值得；若将来等保/合规要求自管密钥，再评估 KMS 实例
- **最小权限**：FC 角色仅持有本数据桶 `Get/Put/Delete/List` 权限；桶私有，客户端永不直接接触 OSS
- **密钥**：`jwt_public_key`（外部子系统验签公钥，公开材料）与 `image` 通过 `TF_VAR_*` 或 terraform.tfvars 注入，**不入库**（tfvars.example 只给占位值）；当前 FC 环境变量携带公钥落入 tfstate，后续迁移 Vault 注入（服务端密钥底座，见 docs/dev-guide/security.md 第 9 章；当前仅预留架构位）
- **恢复**：版本控制保留历史版本，误删/误写可回滚；生命周期 `version-cleanup` 默认 30 天后清理旧版本

## 使用

```sh
terraform init \
  -backend-config="bucket=quanttide-terraform-state" \
  -backend-config="key=qtcloud-secret/terraform.tfstate" \
  -backend-config="region=cn-hangzhou"
terraform plan -var-file=terraform.tfvars
terraform apply -var-file=terraform.tfvars
```

## 待办

- [x] Dockerfile 与镜像发布流水线（双通道：Docker Hub + ACR，`.github/workflows/deploy-provider.yml`，tag `provider/*` 触发）
- [ ] 环境划分（dev / prod）与配置管理（`OSS_BUCKET` / `JWT_PUBLIC_KEY` 等）
- [ ] API 网关统一接入 `api.quanttide.com/qtcloud-secret`（系统层面预留，另行规划）
- [ ] Vault 引入（预留）：服务端密钥底座（OSS STS 动态凭证 / JWT 密钥 / DB 口令 / transit），替代 KMS 3.0 实例方案；等团队版/私有化立项时落地（ECS + Raft 集群），当前不引入
- [ ] 团队版引入 PG 时：RDS 数据库落 `rds.tf`（对齐 qtcloud-delib），密文对象零迁移
