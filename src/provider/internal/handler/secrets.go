// Package handler 实现 /secrets 的 REST 处理器。
//
// 链路（见 docs/dev-guide/transfer.md）：
//
//	客户端密文信封 → POST/PUT → 本服务验签 + 校验 → OSS
//	客户端 ← GET（列表/单个） ← 本服务代理读取
//
// 本服务只代理密文与元数据，不接触明文（零知识红线）。
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/quanttide-secret/provider/internal/auth"
	"github.com/quanttide/quanttide-secret/provider/internal/model"
	"github.com/quanttide/quanttide-secret/provider/internal/storage"
)

const (
	secretsPrefix = "secrets/"
	objectKey     = "id" // URL 路径参数名
)

// Handler 聚合依赖的 REST 处理器。
type Handler struct {
	verifier *auth.Verifier
	store    storage.Store
}

// New 创建处理器。
func New(verifier *auth.Verifier, store storage.Store) *Handler {
	return &Handler{verifier: verifier, store: store}
}

// Routes 注册路由（Go 1.22+ 方法路由），全部经 JWT 验签中间件。
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /secrets", h.list)
	mux.HandleFunc("POST /secrets", h.create)
	mux.HandleFunc("GET /export", h.export)
	mux.HandleFunc("GET /secrets/{id}", h.get)
	mux.HandleFunc("PUT /secrets/{id}", h.update)
	mux.HandleFunc("DELETE /secrets/{id}", h.delete)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return h.verifier.Middleware(mux)
}

// list GET /secrets：返回对象清单（id/updatedAt），客户端全量同步。
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	metas, err := h.store.List(r.Context(), secretsPrefix)
	if err != nil {
		h.audit(r, "list", "", "失败: "+err.Error())
		http.Error(w, "列表失败", http.StatusInternalServerError)
		return
	}
	type item struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	out := make([]item, 0, len(metas))
	for _, m := range metas {
		id := strings.TrimPrefix(m.Key, secretsPrefix)
		out = append(out, item{ID: id, UpdatedAt: m.UpdatedAt})
	}
	h.audit(r, "list", "", "成功")
	writeJSON(w, http.StatusOK, out)
}

// export GET /export：导出全部密文信封（NDJSON 流式）。
//
// 设计（对齐 docs/user-guide/backup-recovery.md「数据本身的备份」）：
//   - 零知识约束：服务端无密钥，只能导出密文信封集合（明文由客户端用主密码解密）
//   - NDJSON 逐行输出：任一对象损坏不影响整体，客户端可逐行解析、部分恢复
//   - 单对象读取失败：跳过并审计，不中断整体导出（版本控制兜底，失败概率极低）
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	metas, err := h.store.List(r.Context(), secretsPrefix)
	if err != nil {
		h.audit(r, "export", "", "失败: "+err.Error())
		http.Error(w, "导出失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="qtcloud-secret-backup.ndjson"`)

	exported, skipped := 0, 0
	for _, m := range metas {
		data, err := h.store.Get(r.Context(), m.Key)
		if err != nil {
			skipped++
			h.audit(r, "export", m.Key, "跳过: "+err.Error())
			continue
		}
		// 原样输出密文信封（不解析、不校验内容——零知识红线）
		if _, err := w.Write(data); err != nil {
			h.audit(r, "export", m.Key, "写入中断: "+err.Error())
			return
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return
		}
		exported++
	}
	h.audit(r, "export", "", fmt.Sprintf("成功 导出=%d 跳过=%d", exported, skipped))
}

// create POST /secrets：校验信封 → PUT OSS。
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	env, body, err := parseBody(w, r)
	if err != nil {
		h.audit(r, "create", "", "校验失败: "+err.Error())
		return
	}
	if err := h.store.Put(r.Context(), secretsPrefix+env.ID, body); err != nil {
		h.audit(r, "create", env.ID, "失败: "+err.Error())
		http.Error(w, "写入失败", http.StatusInternalServerError)
		return
	}
	h.audit(r, "create", env.ID, "成功")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": env.ID, "updatedAt": env.UpdatedAt})
}

// get GET /secrets/{id}：代理读取密文信封。
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue(objectKey)
	data, err := h.store.Get(r.Context(), secretsPrefix+id)
	if err != nil {
		h.audit(r, "get", id, "失败: "+err.Error())
		if err == storage.ErrNotFound {
			http.Error(w, "对象不存在", http.StatusNotFound)
		} else {
			http.Error(w, "读取失败", http.StatusInternalServerError)
		}
		return
	}
	h.audit(r, "get", id, "成功")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// update PUT /secrets/{id}：校验（id 须与路径一致）→ 覆盖写。
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue(objectKey)
	env, body, err := parseBody(w, r)
	if err != nil {
		h.audit(r, "update", id, "校验失败: "+err.Error())
		return
	}
	if env.ID != id {
		h.audit(r, "update", id, "校验失败: id 与路径不一致")
		http.Error(w, "id 与路径不一致", http.StatusBadRequest)
		return
	}
	if err := h.store.Put(r.Context(), secretsPrefix+id, body); err != nil {
		h.audit(r, "update", id, "失败: "+err.Error())
		http.Error(w, "写入失败", http.StatusInternalServerError)
		return
	}
	h.audit(r, "update", id, "成功")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "updatedAt": env.UpdatedAt})
}

// delete DELETE /secrets/{id}：物理删除（OSS delete marker 兜底恢复）。
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue(objectKey)
	if err := h.store.Delete(r.Context(), secretsPrefix+id); err != nil {
		h.audit(r, "delete", id, "失败: "+err.Error())
		if err == storage.ErrNotFound {
			http.Error(w, "对象不存在", http.StatusNotFound)
		} else {
			http.Error(w, "删除失败", http.StatusInternalServerError)
		}
		return
	}
	h.audit(r, "delete", id, "成功")
	w.WriteHeader(http.StatusNoContent)
}

// parseBody 读取并校验请求体（大小上限 + 信封结构）。
func parseBody(w http.ResponseWriter, r *http.Request) (*model.Envelope, []byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, model.MaxEnvelopeSize+1))
	if err != nil {
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return nil, nil, err
	}
	if len(body) > model.MaxEnvelopeSize {
		http.Error(w, "请求体过大", http.StatusRequestEntityTooLarge)
		return nil, nil, errTooLarge
	}
	env, err := model.ParseEnvelope(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, nil, err
	}
	return env, body, nil
}

var errTooLarge = &http.MaxBytesError{}

// audit 审计日志（当前阶段：标准日志输出；团队版/合规要求时落独立审计存储）。
func (h *Handler) audit(r *http.Request, action, id, result string) {
	log.Printf("audit method=%s action=%s id=%s result=%s remote=%s", r.Method, action, id, result, r.RemoteAddr)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
