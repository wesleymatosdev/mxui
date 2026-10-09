// discord-compat: Discord Bot API subset on mxui-server.
// Channel id = Matrix room alias localpart ("demo" → #mxui-demo:server).
// Bearer token = mxui bot token. Components translate Discord v2 JSON → mxui blocks.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"mxui/internal/mxevent"
)

type discordMessage struct {
	Content    string           `json:"content"`
	Components []discordComponent `json:"components"`
}

type discordComponent struct {
	Type       int              `json:"type"` // 1 = action row
	Components []discordElement `json:"components"`
}

type discordElement struct {
	Type     int    `json:"type"` // 2 = button
	Style    int    `json:"style"`
	Label    string `json:"label"`
	CustomID string `json:"custom_id"`
}

func discordStyle(s int) string {
	switch s {
	case 3:
		return "primary"
	case 4:
		return "danger"
	default:
		return "default"
	}
}

func (s *server) resolveChannel(id string) string {
	if strings.HasPrefix(id, "!") {
		return id // raw room id
	}
	if strings.HasPrefix(id, "#") {
		return s.roomID // only the configured room is mapped in v0.5
	}
	return s.roomID // alias localpart → configured room
}

func bearerToken(h http.Header) string {
	a := h.Get("Authorization")
	a = strings.TrimPrefix(a, "Bearer ")
	a = strings.TrimPrefix(a, "Bot ")
	return a
}

func (s *server) handleDiscordPostMessage(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r.Header)
	if !s.botExists(tok) {
		httpError(w, 401, "unknown bot token")
		return
	}
	// path: [.../]v10/channels/{id}/messages — prefix-agnostic
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[len(parts)-1] != "messages" || parts[len(parts)-3] != "channels" {
		httpError(w, 404, "unsupported route (v0.5 slice: POST/PATCH messages only)")
		return
	}
	room := s.resolveChannel(parts[len(parts)-2])
	var dm discordMessage
	if err := json.NewDecoder(r.Body).Decode(&dm); err != nil {
		httpError(w, 400, "bad json")
		return
	}
	var blocks []mxevent.ComponentBlock
	for _, row := range dm.Components {
		var comps []mxevent.Component
		for _, el := range row.Components {
			if el.Type == 2 {
				comps = append(comps, mxevent.Component{
					ID: el.CustomID, Type: "button", Label: el.Label,
					Style: discordStyle(el.Style), Value: el.CustomID,
				})
			}
		}
		if len(comps) > 0 {
			blocks = append(blocks, mxevent.ComponentBlock{BlockID: "row", Components: comps})
		}
	}
	payload := mxevent.MessagePayload{Body: dm.Content, Blocks: blocks}
	var id string
	var err error
	if len(blocks) > 0 {
		id, err = s.matrix.SendInteractive(room, payload)
	} else {
		err = s.matrix.SendPlain(room, dm.Content)
	}
	if err != nil {
		httpError(w, 502, err.Error())
		return
	}
	if len(blocks) > 0 {
		s.mu.Lock()
		s.msgByBot[tok] = id
		s.mu.Unlock()
	}
	writeJSON(w, map[string]any{"id": id, "channel_id": parts[len(parts)-2], "content": dm.Content})
}

func (s *server) handleDiscordPatchMessage(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r.Header)
	if !s.botExists(tok) {
		httpError(w, 401, "unknown bot token")
		return
	}
	// path: [.../]v10/channels/{id}/messages/{mid} — prefix-agnostic
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[len(parts)-2] != "messages" || parts[len(parts)-4] != "channels" {
		httpError(w, 404, "unsupported route")
		return
	}
	room := s.resolveChannel(parts[len(parts)-3])
	var dm discordMessage
	if err := json.NewDecoder(r.Body).Decode(&dm); err != nil {
		httpError(w, 400, "bad json")
		return
	}
	mid := parts[len(parts)-1]
	if err := s.matrix.EditMessage(room, mid, dm.Content); err != nil {
		httpError(w, 502, err.Error())
		return
	}
	writeJSON(w, map[string]any{"id": mid, "content": dm.Content})
}

// forwardDiscordInteraction wraps an mxui action in Discord's interaction shape
// and POSTs it to every registered bot's webhook (Discord HTTP-interactions style).
func (s *server) forwardDiscordInteraction(a mxevent.ActionEvent) {
	s.mu.Lock()
	rows, err := s.db.Query(`SELECT webhook_url FROM bots`)
	s.mu.Unlock()
	if err != nil {
		return
	}
	var hooks []string
	for rows.Next() {
		var h string
		if rows.Scan(&h) == nil {
			hooks = append(hooks, h)
		}
	}
	rows.Close()
	payload := map[string]any{
		"type": 3, // MESSAGE_COMPONENT
		"token": fmt.Sprintf("mxui-interaction-%d", a.TS),
		"data": map[string]any{
			"custom_id":     a.ComponentID,
			"component_type": 2,
			"value":         a.Value,
		},
		"room_id":    a.RoomID,
		"message":    map[string]any{"id": a.MessageEvent},
		"user":       map[string]any{"id": a.UserID, "username": a.UserID},
		"mxui_echo":  a,
	}
	b, _ := json.Marshal(payload)
	for _, hook := range hooks {
		go func(hook string) {
			req, _ := http.NewRequest("POST", hook, strings.NewReader(string(b)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Signature-Ed25519", "disabled-local") // local dev only
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}(hook)
	}
}

func (s *server) botExists(tok string) bool {
	if tok == "" {
		return false
	}
	var one int
	return s.db.QueryRow(`SELECT 1 FROM bots WHERE token = ?`, tok).Scan(&one) == nil
}
