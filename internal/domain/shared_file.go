package domain

import "time"

// SharedFile は Backlog の共有ファイルモデル。
// Backlog API: GET /api/v2/projects/{projectIdOrKey}/files/metadata/{path}
type SharedFile struct {
	ID          int64      `json:"id"`
	Type        string     `json:"type"`
	Dir         string     `json:"dir"`
	Name        string     `json:"name"`
	Size        int64      `json:"size"`
	CreatedUser *User      `json:"createdUser,omitempty"`
	Created     *time.Time `json:"created,omitempty"`
	UpdatedUser *User      `json:"updatedUser,omitempty"`
	Updated     *time.Time `json:"updated,omitempty"`
}
