// Package config 从环境变量加载服务端配置。
//
// 环境变量约定（与 manifests/terraform/fc.tf 一致）：
//
//	OSS_BUCKET   密文数据桶名
//	OSS_ENDPOINT OSS endpoint（如 https://oss-cn-hangzhou.aliyuncs.com）
//	JWT_SECRET   与 qtcloud-auth 共享的 JWT HS256 签名密钥（org secret）
//	PORT         监听端口（默认 8080，FC custom-container 约定）
package config

import (
	"fmt"
	"os"
)

// Config 服务端运行配置。
type Config struct {
	OSSBucket   string
	OSSEndpoint string
	JWTSecret   []byte // JWT HS256 签名密钥（与 qtcloud-auth 共享）
	Port        string
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

	// JWT 密钥与 qtcloud-auth 共享（见 qtcloud-auth 的 JWT_SECRET 注入方式）
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("环境变量 JWT_SECRET 未设置")
	}
	cfg.JWTSecret = []byte(secret)

	return cfg, nil
}
