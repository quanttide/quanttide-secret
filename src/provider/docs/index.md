# 服务端设计方案（provider）

> 本文档明确 qtcloud-secret 服务端（src/provider，Go 实现）的设计方案。
> 架构总纲见仓库 `docs/dev-guide/`：storage.md（存储）、transfer.md（传输）、model.md（数据模型）、security.md（零知识与 Vault 边界）。

## 1. 定位与职责边界

服务端是一个**薄代理层**：验签 → 校验 → 代理 OSS 读写 → 审计。它不参与任何加密逻辑。

| 职责 | 说明 |
|------|------|
| ✅ 认证 | 验签外部子系统签发的 JWT（无状态，不建账号不存会话） |
| ✅ 校验 | 对象 key 必须为 UUID v4、信封大小 ≤ 64 KB、信封外层结构校验 |
| ✅ 代理读写 | 密文信封的 Put / Get / Delete / List（客户端永不直接接触 OSS） |
| ✅ 审计 | 每次读写记录操作者、时间、对象 id、结果 |
| ❌ 加密/解密 | **绝不接触明文与客户端密钥**（零知识红线，见 security.md） |
| ❌ 用户管理 | 用户在外部子系统，本服务不存用户 |

**一句话：服务端经手所有密文，但对密文内容一无所知——密文对它只是无意义字节。**

## 2. 部署形态

| 项 | 选型 | 说明 |
|----|------|------|
| 运行环境 | 阿里云函数计算 FC 3.0（custom-container） | 按调用计费、无需常驻；容器监听 8080（Dockerfile 已就绪） |
| 存储 | 阿里云 OSS 单桶 | 版本控制 + SSE-OSS 二次加密 + 生命周期清理旧版本（manifests/terraform/oss.tf） |
| 数据库 | **当前阶段无** | 单团队无 RBAC 关系需求；团队版引入 PG 时密文零迁移 |
| 密钥管理 | **预留（HashiCorp Vault）** | 服务端密钥底座（OSS 动态凭证/JWT 密钥/DB 口令/transit），等团队版/私有化立项引入 |
| 网关 | 预留 | 系统级 API 网关统一接入 `api.quanttide.com/qtcloud-secret` |

## 3. 认证设计

```
外部子系统 ──签发 JWT (RS256/ES256)──> 客户端
客户端 ──Authorization: Bearer <JWT>──> 本服务
本服务：公钥验签（JWKS/静态公钥）→ 校验 exp/aud/iss → 放行
```

- 公钥经环境变量 `JWT_PUBLIC_KEY`（base64 PEM）注入，支持 RSA 与 ECDSA（internal/auth/jwt.go）
- 每请求无状态验签，不建 session；短过期 + 时间窗口防重放
- 授权规则：当前阶段验签通过即可读写（单团队）；JWT `scope` 字段预留团队版细粒度权限

## 4. API 设计

Base path：`/secrets`。全部端点经 JWT 验签中间件（无版本前缀，小服务不引入版本化复杂度）。

| 方法 | 路径 | 说明 | 成功响应 |
|------|------|------|---------|
| GET | `/secrets` | 对象清单（id/updatedAt），客户端全量同步 | 200 JSON 数组 |
| POST | `/secrets` | 创建（校验信封 → PUT OSS） | 201 {id, updatedAt} |
| GET | `/secrets/{id}` | 读取密文信封（代理） | 200 信封 JSON |
| GET | `/export` | 导出全部密文信封（NDJSON 流式，离线备份/迁移） | 200 NDJSON |
| PUT | `/secrets/{id}` | 更新（覆盖写，id 须与路径一致） | 200 {id, updatedAt} |
| DELETE | `/secrets/{id}` | 删除（OSS delete marker 兜底恢复） | 204 |
| GET | `/health` | 健康检查 | 200 |

> `export` 为字面量路径，匹配优先于 `{id}` 通配。

### 导出备份说明

服务端无主密码，只能导出**密文信封集合**（明文导出是客户端职责：本地解密后导出）：

- NDJSON 逐行输出，任一对象损坏不影响整体（单对象读取失败跳过并审计）
- 用途：客户端离线保管（配合主密码解密）、换设备迁移、最终保险
- 与服务端侧 OSS 版本控制互补，见 docs/user-guide/backup-recovery.md

错误码：`400` 校验失败 / `401` 令牌无效 / `404` 对象不存在 / `413` 请求体过大 / `500` 存储异常。

请求示例（信封结构对齐 model.md，密文负载对服务端不透明）：

```json
{
  "id": "a1b2c3d4-0000-4000-8000-000000000000",
  "name": "GitHub 登录",
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-01-01T00:00:00Z",
  "encrypted": {
    "algorithm": "AES-256-GCM",
    "kdf": "Argon2id",
    "kdfSalt": "base64...",
    "nonce": "base64...",
    "ciphertext": "base64..."
  }
}
```

## 5. 存储设计

```
oss://qtcloud-secret-data/          # 桶：版本控制 + SSE-OSS + 生命周期
  secrets/<uuid-v4>.json            # 每个密钥一个密文信封对象
```

- 对象 key 为不可预测 UUID，路径不含明文信息
- 写 = 覆盖（版本控制保留历史）；删 = 物理删除（delete marker 可恢复）
- 同步 = 全量 List + 差异拉取（小数据量，无需增量协议）
- 服务端经 RAM 角色（STS 临时凭证）访问，最小权限仅本桶 `Get/Put/Delete/List`（manifests/terraform/fc.tf）

## 6. 请求校验规则（服务端）

| 规则 | 值 | 说明 |
|------|-----|------|
| 对象 key | UUID v4 格式 | 拒绝路径穿越与非法 key |
| 请求体大小 | ≤ 64 KB | 密文信封体积上限 |
| 信封结构 | id/name/encrypted 必填、base64 字段格式 | 仅外层校验，不解析密文内容 |

## 7. 安全机制

| 机制 | 实现 |
|------|------|
| 传输加密 | 全链路 TLS（FC 触发器 + 网关） |
| 令牌验签 | JWT RS256/ES256，公钥验签，无状态 |
| OSS 访问 | STS 最小权限角色，客户端永不直连 |
| 零知识 | 服务端无密钥可接触明文；泄露兜底见 security.md 第 6 章 |
| 审计 | 标准日志输出（团队版/合规要求时落独立审计存储） |
| 错误信息 | 对外不泄露内部细节（对象不存在 vs 存储异常区分返回） |

## 8. 代码结构

```
src/provider/
├── cmd/server/main.go          # 入口：配置加载、依赖组装、启动 HTTP
├── internal/
│   ├── config/config.go        # 环境变量配置（OSS_BUCKET/OSS_ENDPOINT/JWT_PUBLIC_KEY/PORT）
│   ├── auth/jwt.go             # JWT 验签（RSA/ECDSA）+ 中间件
│   ├── model/envelope.go       # 密文信封结构 + 外层校验
│   ├── storage/storage.go      # Store 接口（Put/Get/Delete/List）
│   ├── storage/oss.go          # 阿里云 OSS 实现（SDK）
│   └── handler/secrets.go      # REST 处理器 + 审计
├── docs/index.md               # 本文档
├── Dockerfile                  # 多阶段构建，非 root，监听 8080
└── go.mod / go.sum
```

## 9. 演进预留（当前不实现）

| 未来需求 | 预留方式 |
|----------|---------|
| 团队版 RBAC | JWT `scope` 字段；PG 关系层引入（manifests rds.tf） |
| Vault 服务端密钥底座 | 环境变量注入点已抽象；FC 凭据改 Vault Agent 注入 |
| 大附件 | 届时引入 presigned URL 直传（本服务签发，当前代理链路不变） |
| 独立审计存储 | audit() 已集中，可平滑替换输出目标 |
| 配置中心 | config.Load() 已集中，可平滑切换来源 |
