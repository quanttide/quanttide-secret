// Package config 从环境变量加载服务端配置。
//
// 环境变量约定（与 manifests/terraform/fc.tf 一致）：
//
//	OSS_BUCKET     密文数据桶名
//	OSS_ENDPOINT   OSS endpoint（如 https://oss-cn-hangzhou.aliyuncs.com）
//	JWT_PUBLIC_KEY 外部子系统 JWT 验签公钥（base64 编码 PEM，RS256/ES256）
//	PORT           监听端口（默认 8080，FC custom-container 约定）
package config

import (
	"encoding/base64"
	"fmt"
	"os"
)

// Config 服务端运行配置。
type Config struct {
	OSSBucket    string
	OSSEndpoint  string
	JWTPublicKey []byte // 已解码的 PEM 公钥
	Port         string
}

// Load 从环境变量加载配置并校验必填项。
func Load() (*Config, error) {
	cfg := &Config{
		OSSBucket:   os.Getenv("OSS_BUCKET"),
		OSSEndpoint: os.Getenv("OSS_ENDPOINT"),
		Port:        os.Getenv("PORT"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	if cfg.OSSBucket == "" {
		return nil, fmt.Errorf("环境变量 OSS_BUCKET 未设置")
	}
	if cfg.OSSEndpoint == "" {
		return nil, fmt.Errorf("环境变量 OSS_ENDPOINT 未设置")
	}

	// JWT 公钥以 base64(PEM) 注入（见 manifests/terraform/fc.tf 与 README 安全说明）
	encoded := os.Getenv("JWT_PUBLIC_KEY")
	if encoded == "" {
		return nil, fmt.Errorf("环境变量 JWT_PUBLIC_KEY 未设置")
	}
	pem, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("JWT_PUBLIC_KEY 不是合法 base64: %w", err)
	}
	cfg.JWTPublicKey = pem

	return cfg, nil
}
