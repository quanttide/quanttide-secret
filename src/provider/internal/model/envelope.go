// Package model 定义密文信封对象结构（对齐 docs/dev-guide/model.md）。
//
// 服务端只做外层结构与格式校验，绝不解析 encrypted 内部内容——
// 那是客户端零知识的领地（见 docs/dev-guide/security.md）。
package model

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Envelope 每个密钥一个 JSON 信封对象：明文元数据 + 密文负载。
type Envelope struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Encrypted EncryptedPayload `json:"encrypted"`
}

// EncryptedPayload 客户端加密负载（服务端视为不透明字节）。
type EncryptedPayload struct {
	Algorithm  string `json:"algorithm"`
	KDF        string `json:"kdf"`
	KDFSalt    string `json:"kdfSalt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// 校验规则（见 docs/index.md「请求校验」）。
const (
	MaxEnvelopeSize = 64 * 1024 // 信封大小上限 64 KB（含 base64 密文）
	MaxNameLength   = 256       // name 长度上限
)

var (
	uuidV4Re = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	base64Re = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
)

// Validate 校验信封的外层结构（不解密、不接触内容）。
func (e *Envelope) Validate() error {
	if !uuidV4Re.MatchString(e.ID) {
		return errors.New("id 必须是 UUID v4")
	}
	if e.Name == "" || len(e.Name) > MaxNameLength {
		return fmt.Errorf("name 长度必须在 1-%d 之间", MaxNameLength)
	}
	if e.Encrypted.Algorithm == "" {
		return errors.New("encrypted.algorithm 不能为空")
	}
	if e.Encrypted.KDF == "" {
		return errors.New("encrypted.kdf 不能为空")
	}
	for field, v := range map[string]string{
		"kdfSalt":    e.Encrypted.KDFSalt,
		"nonce":      e.Encrypted.Nonce,
		"ciphertext": e.Encrypted.Ciphertext,
	} {
		if v == "" || !base64Re.MatchString(v) {
			return fmt.Errorf("encrypted.%s 必须是 base64 字符串", field)
		}
	}
	if n, err := base64.StdEncoding.DecodeString(e.Encrypted.Ciphertext); err == nil && len(n) == 0 {
		return errors.New("encrypted.ciphertext 为空")
	}
	return nil
}

// ParseEnvelope 解析并校验请求体（含大小上限）。
func ParseEnvelope(body []byte) (*Envelope, error) {
	if len(body) == 0 || len(body) > MaxEnvelopeSize {
		return nil, fmt.Errorf("请求体大小必须在 1-%d 字节之间", MaxEnvelopeSize)
	}
	var e Envelope
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, fmt.Errorf("非法 JSON: %w", err)
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return &e, nil
}
