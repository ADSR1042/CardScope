package hub

import (
	"errors"
	"net/http"
	"strings"
)

func nodeFailure(w http.ResponseWriter, err error, fallback string) {
	var input nodeInputError
	switch {
	case errors.As(err, &input):
		fail(w, http.StatusBadRequest, input.Error())
	case errors.Is(err, errNodeNotFound):
		fail(w, http.StatusNotFound, err.Error())
	default:
		fail(w, http.StatusInternalServerError, fallback)
	}
}

func (h *Hub) createNode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if decode(w, r, &body, 4096) != nil {
		fail(w, 400, "节点名称无效")
		return
	}
	result, err := (nodeService{h.DB}).create(body.Name)
	if err != nil {
		nodeFailure(w, err, "创建失败")
		return
	}
	respond(w, 201, result)
}

func (h *Hub) updateNode(w http.ResponseWriter, r *http.Request, path string) {
	service := nodeService{h.DB}
	parts := strings.Split(strings.TrimPrefix(path, "/nodes/"), "/")
	id := parts[0]
	if err := service.requireNode(id); err != nil {
		nodeFailure(w, err, "节点查询失败")
		return
	}
	if len(parts) == 2 && r.Method == "POST" {
		switch parts[1] {
		case "revoke":
			if err := service.revoke(id); err != nil {
				nodeFailure(w, err, "更新失败")
				return
			}
			respond(w, 200, map[string]bool{"ok": true})
		case "enrollment":
			code, err := service.issueCode(id)
			if err != nil {
				nodeFailure(w, err, "更新失败")
				return
			}
			respond(w, 200, map[string]string{"code": code})
		default:
			fail(w, 404, "接口不存在")
		}
		return
	}
	if len(parts) == 1 && r.Method == "PATCH" {
		var settings nodeSettings
		if decode(w, r, &settings, 8192) != nil {
			fail(w, 400, "配置无效")
			return
		}
		if err := service.configure(id, settings); err != nil {
			nodeFailure(w, err, "保存失败")
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	fail(w, 404, "接口不存在")
}
