// Package storage 定义密文对象存储接口与 OSS 实现。
//
// 设计（见 docs/dev-guide/model.md）：
//   - 对象布局 secrets/<id>.json，key 为 UUID v4，路径不含明文信息
//   - 桶开启版本控制（误删/误写可回滚）与 SSE-OSS（第二层加密）
//   - 服务端仅代理读写；客户端永不直接接触 OSS
package storage

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound 对象不存在。
var ErrNotFound = errors.New("对象不存在")

// ObjectMeta 对象摘要（列表/同步用）。
type ObjectMeta struct {
	Key       string // secrets/<id>.json
	Size      int64
	UpdatedAt time.Time
}

// Store 密文对象存储接口。
type Store interface {
	// Put 覆盖写（OSS 版本控制保留历史版本）。
	Put(ctx context.Context, key string, data []byte) error
	// Get 读取密文对象。
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete 物理删除（OSS 生成 delete marker，可恢复）。
	Delete(ctx context.Context, key string) error
	// List 列出前缀下全部对象摘要（客户端全量同步用）。
	List(ctx context.Context, prefix string) ([]ObjectMeta, error)
}
