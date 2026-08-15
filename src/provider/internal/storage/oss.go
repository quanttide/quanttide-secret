package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSStore 基于阿里云 OSS 的 Store 实现。
//
// 凭证来源：FC 运行时注入的 RAM 角色（STS 临时凭证，见 manifests/terraform/fc.tf），
// SDK 自动从环境变量读取；本地开发可用静态 AK/SK 环境变量。
type OSSStore struct {
	bucket *oss.Bucket
}

// NewOSSStore 创建 OSS 存储客户端。
func NewOSSStore(bucketName, endpoint string) (*OSSStore, error) {
	client, err := oss.New(endpoint, "", "")
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %w", err)
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, fmt.Errorf("获取 OSS 桶失败: %w", err)
	}
	return &OSSStore{bucket: bucket}, nil
}

// key 规整：确保 secrets/ 前缀与无前导斜杠。
func keyOf(id string) string {
	if !strings.HasPrefix(id, "secrets/") {
		return "secrets/" + id
	}
	return id
}

func (s *OSSStore) Put(ctx context.Context, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.bucket.PutObject(keyOf(key), bytes.NewReader(data))
}

func (s *OSSStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := s.bucket.GetObject(keyOf(key))
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("读取对象失败: %w", err)
	}
	defer body.Close()
	return io.ReadAll(body)
}

func (s *OSSStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.bucket.DeleteObject(keyOf(key)); err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("删除对象失败: %w", err)
	}
	return nil
}

func (s *OSSStore) List(ctx context.Context, prefix string) ([]ObjectMeta, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var metas []ObjectMeta
	marker := ""
	for {
		resp, err := s.bucket.ListObjects(oss.Prefix(prefix), oss.Marker(marker), oss.MaxKeys(1000))
		if err != nil {
			return nil, fmt.Errorf("列出对象失败: %w", err)
		}
		for _, obj := range resp.Objects {
			metas = append(metas, ObjectMeta{
				Key:       obj.Key,
				Size:      obj.Size,
				UpdatedAt: obj.LastModified,
			})
		}
		if !resp.IsTruncated {
			break
		}
		marker = resp.NextMarker
	}
	return metas, nil
}

func isNotFound(err error) bool {
	if ossErr, ok := err.(oss.ServiceError); ok {
		return ossErr.StatusCode == 404
	}
	return false
}
