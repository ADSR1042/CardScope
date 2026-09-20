package hub

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var errNodeNotFound = errors.New("节点不存在")

type nodeInputError string

func (e nodeInputError) Error() string { return string(e) }

type nodeSettings struct {
	Name     string   `json:"name"`
	Expected int      `json:"expected"`
	Networks []string `json:"networks"`
	Tags     []string `json:"tags"`
}

type nodeEnrollment struct {
	NodeID string `json:"node_id"`
	Code   string `json:"code"`
}

// nodeService implements node management without depending on HTTP.
// Callers hold Hub.mu across operations to serialize credential changes with
// enrollment and snapshot ingestion.
type nodeService struct{ db *sql.DB }

func (s nodeService) requireNode(id string) error {
	var exists int
	err := s.db.QueryRow("SELECT 1 FROM nodes WHERE id=?", id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return errNodeNotFound
	}
	return err
}

func (s nodeService) create(name string) (nodeEnrollment, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return nodeEnrollment{}, nodeInputError("节点名称无效")
	}
	if err := s.uniqueName(name, ""); err != nil {
		return nodeEnrollment{}, err
	}
	result := nodeEnrollment{NodeID: token()[:24], Code: token()}
	now := time.Now()
	_, err := s.db.Exec("INSERT INTO nodes(id,name,code_hash,code_expires,created) VALUES(?,?,?,?,?)",
		result.NodeID, name, hash(result.Code), now.Add(15*time.Minute).UnixMilli(), now.UnixMilli())
	return result, err
}

func (s nodeService) configure(id string, settings nodeSettings) error {
	settings.Name = strings.TrimSpace(settings.Name)
	if settings.Name == "" || len(settings.Name) > 128 || settings.Expected < 0 || settings.Expected > 128 || len(settings.Networks) > 64 {
		return nodeInputError("配置无效")
	}
	if err := s.uniqueName(settings.Name, id); err != nil {
		return err
	}
	for _, name := range settings.Networks {
		if len(name) > 128 {
			return nodeInputError("网卡名称过长")
		}
	}
	tags := []string{}
	seen := map[string]bool{}
	if len(settings.Tags) > 20 {
		return nodeInputError("每个节点最多 20 个标签")
	}
	for _, tag := range settings.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || utf8.RuneCountInString(tag) > 32 || strings.ContainsFunc(tag, unicode.IsControl) {
			return nodeInputError("标签须为 1～32 个字符，不能包含控制字符")
		}
		if !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}
	// Older clients omit tags; preserve them instead of silently clearing them.
	if settings.Tags != nil {
		result, err := s.db.Exec("UPDATE nodes SET name=?,expected=?,networks=?,tags=? WHERE id=?",
			settings.Name, settings.Expected, marshal(settings.Networks), marshal(tags), id)
		return nodeUpdateResult(result, err)
	}
	result, err := s.db.Exec("UPDATE nodes SET name=?,expected=?,networks=? WHERE id=?",
		settings.Name, settings.Expected, marshal(settings.Networks), id)
	return nodeUpdateResult(result, err)
}

// Management callers hold Hub.mu, so the name check and write are serialized.
// Compare legacy names after trimming as well; never rename existing nodes on upgrade.
func (s nodeService) uniqueName(name, exceptID string) error {
	rows, err := s.db.Query("SELECT name FROM nodes WHERE id<>?", exceptID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var existing string
		if err := rows.Scan(&existing); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(existing), name) {
			return nodeInputError("节点名称已存在")
		}
	}
	return rows.Err()
}

func (s nodeService) revoke(id string) error {
	result, err := s.db.Exec("UPDATE nodes SET token_hash='',code_hash='',code_expires=0 WHERE id=?", id)
	return nodeUpdateResult(result, err)
}

func (s nodeService) issueCode(id string) (string, error) {
	code := token()
	result, err := s.db.Exec("UPDATE nodes SET token_hash='',code_hash=?,code_expires=? WHERE id=?",
		hash(code), time.Now().Add(15*time.Minute).UnixMilli(), id)
	return code, nodeUpdateResult(result, err)
}

func nodeUpdateResult(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return errNodeNotFound
	}
	return err
}
