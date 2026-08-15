// Command server 是 qtcloud-secret 的服务端（provider）。
//
// 部署形态：阿里云函数计算 FC 3.0 custom-container（监听 8080）。
// 职责（见 docs/index.md）：验签 JWT → 校验请求 → 代理 OSS 读写 → 审计日志。
// 零知识红线：本服务只处理密文信封，永不接触明文与客户端密钥。
package main

import (
	"log"
	"net/http"

	"github.com/quanttide/quanttide-secret/provider/internal/auth"
	"github.com/quanttide/quanttide-secret/provider/internal/config"
	"github.com/quanttide/quanttide-secret/provider/internal/handler"
	"github.com/quanttide/quanttide-secret/provider/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	verifier, err := auth.NewJWTVerifier(cfg.JWTPublicKey)
	if err != nil {
		log.Fatalf("初始化 JWT 验签失败: %v", err)
	}

	store, err := storage.NewOSSStore(cfg.OSSBucket, cfg.OSSEndpoint)
	if err != nil {
		log.Fatalf("初始化 OSS 存储失败: %v", err)
	}

	h := handler.New(verifier, store)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: h.Routes(),
	}
	log.Printf("qtcloud-secret provider 启动，监听 :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}
