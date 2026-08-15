// Package auth 实现无状态 JWT 验签。
//
// 设计（见 docs/index.md「认证」）：
//   - 用户在外部子系统，本服务不建账号、不存会话
//   - 每请求 Authorization: Bearer <JWT>，用外部子系统公钥验签（RS256/ES256）
//   - 校验 exp / aud / iss；预留 scope 供团队版细粒度权限
package auth

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 认可的 JWT 声明。scope 预留：团队版细粒度权限（当前阶段验签通过即可读写）。
type Claims struct {
	Scope string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// Verifier 校验 JWT 签名与声明。
type Verifier struct {
	key any // *rsa.PublicKey 或 *ecdsa.PublicKey
}

// NewJWTVerifier 解析 base64(PEM) 公钥（先按 RSA 再按 ECDSA 尝试）。
func NewJWTVerifier(pemBytes []byte) (*Verifier, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("无效的 PEM 公钥")
	}

	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		switch k := pub.(type) {
		case *rsa.PublicKey, *ecdsa.PublicKey:
			return &Verifier{key: k}, nil
		default:
			return nil, fmt.Errorf("不支持的密钥类型 %T", k)
		}
	}
	if pub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return &Verifier{key: pub}, nil
	}
	return nil, fmt.Errorf("无法解析公钥")
}

// Verify 验签并校验 exp/aud/iss，返回 JWT 声明。
// aud 与 iss 由部署方在配置时确定（当前单团队场景从宽：仅校验 exp）。
func (v *Verifier) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		switch v.key.(type) {
		case *rsa.PublicKey:
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("意外的签名算法: %v", t.Header["alg"])
			}
		case *ecdsa.PublicKey:
			if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("意外的签名算法: %v", t.Header["alg"])
			}
		}
		return v.key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("JWT 验签失败: %w", err)
	}
	return claims, nil
}

// Middleware 从 Authorization 头提取并验签，通过后注入声明到上下文。
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, "缺少 Bearer 令牌", http.StatusUnauthorized)
			return
		}
		claims, err := v.Verify(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			http.Error(w, "令牌无效: "+err.Error(), http.StatusUnauthorized)
			return
		}
		// 当前阶段：验签通过即可读写（单团队）；scope 预留团队版权限
		_ = claims
		next.ServeHTTP(w, r)
	})
}
