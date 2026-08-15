// Package auth 实现无状态 JWT 验签。
//
// 设计（见 docs/index.md「认证」）：
//   - 用户在外部子系统（qtcloud-auth），本服务不建账号、不存会话
//   - 每请求 Authorization: Bearer <JWT>，用与 qtcloud-auth 共享的 JWT_SECRET 验签（HS256）
//   - 校验 exp；预留 scope 供团队版细粒度权限
//
// 安全说明：HS256 为对称签名，验签方持有密钥即可签发——本服务与 qtcloud-auth
// 同属量潮体系、互相信任（服务端本来就不接触用户明文，见 security.md 零知识边界）。
package auth

import (
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

// Verifier 校验 JWT 签名与声明（HS256，与 qtcloud-auth 共享密钥）。
type Verifier struct {
	secret []byte
}

// NewJWTVerifier 使用与 qtcloud-auth 共享的 JWT_SECRET 创建验签器（HS256）。
func NewJWTVerifier(secret []byte) (*Verifier, error) {
	if len(secret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET 长度不足（至少 16 字节）")
	}
	return &Verifier{secret: secret}, nil
}

// Verify 验签并校验 exp，返回 JWT 声明。
func (v *Verifier) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名算法: %v", t.Header["alg"])
		}
		return v.secret, nil
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
