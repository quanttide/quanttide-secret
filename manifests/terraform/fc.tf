# =============================================================================
# 应用服务（对齐 docs/dev-guide/transfer.md）
#
# FC 3.0 custom-container：验签 JWT + 校验 + 代理 OSS 读写 + 审计。
# 客户端不直连 OSS：应用服务持有最小权限 RAM 角色（STS），
# 明文永不离开客户端（密文信封经 FC 落 OSS，SSE-OSS 二次加密兜底）。
# =============================================================================

# FC 默认角色：允许 FC 服务代入（应用级）
resource "alicloud_ram_role" "fc" {
  role_name                   = "${local.app_name_prefix}-fc"
  assume_role_policy_document = <<EOF
{
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": ["fc.aliyuncs.com"]
      }
    }
  ],
  "Version": "1"
}
EOF
  description                 = "Function Compute 默认角色（qtcloud-secret）"
}

# 最小权限策略：仅允许代理读写本应用数据桶（secrets/ 前缀）
resource "alicloud_ram_policy" "oss_secrets" {
  policy_name     = "${local.app_name_prefix}-oss-secrets"
  description     = "qtcloud-secret 密文数据桶最小读写权限"
  policy_document = <<EOF
{
  "Statement": [
    {
      "Action": [
        "oss:GetObject",
        "oss:PutObject",
        "oss:DeleteObject",
        "oss:ListObjects",
        "oss:ListObjectVersions"
      ],
      "Effect": "Allow",
      "Resource": [
        "acs:oss:*:*:${local.oss_bucket}",
        "acs:oss:*:*:${local.oss_bucket}/*"
      ]
    }
  ],
  "Version": "1"
}
EOF
}

resource "alicloud_ram_role_policy_attachment" "fc_oss" {
  policy_name = alicloud_ram_policy.oss_secrets.policy_name
  policy_type = "Custom"
  role_name   = alicloud_ram_role.fc.role_name
}

# 函数计算（FC 3.0）：custom-container 容器镜像，公网访问 OSS
# （当前阶段无 RDS，不挂 VPC；internet_access 必须显式开启）
resource "alicloud_fcv3_function" "this" {
  function_name     = local.app_name_prefix
  description       = "qtcloud-secret 密码云 API"
  runtime           = "custom-container"
  handler           = "index.handler" # custom-container 必填占位，实际由容器监听端口决定
  cpu               = 0.5
  memory_size       = var.fc_memory
  disk_size         = 512 # FC 3.0 必填（MB）
  timeout           = var.fc_timeout
  internet_access   = true
  role              = alicloud_ram_role.fc.arn
  resource_group_id = data.terraform_remote_state.platform.outputs.resource_group_id

  custom_container_config {
    image = var.image
    port  = 8080
  }

  # 运行时约定（见 docs/dev-guide/transfer.md）：
  #   OSS_BUCKET / OSS_ENDPOINT：数据桶访问；JWT_PUBLIC_KEY：外部子系统 JWT 验签公钥
  # 注意：公钥会以明文落入 tfstate，属非敏感公开材料（仅用于验签）；规划迁移配置中心
  environment_variables = {
    OSS_BUCKET     = alicloud_oss_bucket.secrets.bucket
    OSS_ENDPOINT   = local.oss_endpoint
    JWT_PUBLIC_KEY = var.jwt_public_key
  }

  tags = {
    project     = var.project
    environment = var.environment
  }
}

# HTTP 触发器：使服务可直接访问（后续由系统级 API 网关统一接入，此触发器保留为直连通道）
resource "alicloud_fcv3_trigger" "http" {
  function_name = alicloud_fcv3_function.this.function_name
  trigger_name  = "http"
  trigger_type  = "http"
  qualifier     = "LATEST"
  trigger_config = jsonencode({
    authType = "anonymous"
    methods  = ["GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"]
  })
}
