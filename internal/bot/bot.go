// Package bot is the mxui bot-side client SDK: a thin Matrix client (sync,
// send, edit) plus the interactive-message composer. It deliberately uses only
// the plain CS API — no SDK dependency — so the E2EE v2 path can swap in
// vodozemac-based crypto without re-plumbing.
package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"mxui/internal/mxevent"
)

// Client talks to a Matrix homeserver as one bot user.
type Client struct {
	Homeserver string // e.g. http://localhost:6167
	AccessToken string
	UserID      string
	HTTP        *http.Client
	Since       string // sync cursor
}

func New(homeserver, accessToken string) *Client {
	return &Client{Homeserver: homeserver, AccessToken: accessToken, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) do(method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.Homeserver+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, string(b))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

// Login with password (MVP convenience; registration must be open on the HS).
func (c *Client) Login(user, pass string) error {
	var out struct {
		AccessToken string `json:"access_token"`
		UserID      string `json:"user_id"`
	}
	err := c.do("POST", "/_matrix/client/v3/login", map[string]any{
		"type": "m.login.password",
		"identifier": map[string]any{"type": "m.id.user", "user": user},
		"password":   pass,
	}, &out)
	if err != nil {
		return err
	}
	c.AccessToken, c.UserID = out.AccessToken, out.UserID
	return nil
}

// Register creates a new user via registration token (MXUI_REG_TOKEN env).
func (c *Client) Register(user, pass string) error {
	token := os.Getenv("MXUI_REG_TOKEN")
	if token == "" {
		return fmt.Errorf("MXUI_REG_TOKEN not set")
	}
	var dummy struct{}
	return c.do("POST", "/_matrix/client/v3/register?kind=user", map[string]any{
		"username": user, "password": pass,
		"auth": map[string]string{"type": "m.login.registration_token", "token": token},
	}, &dummy)
}

// JoinRoom by alias or ID.
func (c *Client) JoinRoom(roomIDOrAlias string) (string, error) {
	var out struct {
		RoomID string `json:"room_id"`
	}
	err := c.do("POST", "/_matrix/client/v3/join/"+url.PathEscape(roomIDOrAlias), struct{}{}, &out)
	return out.RoomID, err
}

// CreateRoom makes a new room (public, with alias localpart) and joins it.
func (c *Client) CreateRoom(aliasLocalpart string) (string, error) {
	var out struct {
		RoomID string `json:"room_id"`
	}
	body := map[string]any{
		"preset": "public_chat",
		"name":   "mxui demo",
	}
	if aliasLocalpart != "" {
		body["room_alias_name"] = aliasLocalpart
	}
	err := c.do("POST", "/_matrix/client/v3/createRoom", body, &out)
	return out.RoomID, err
}

// SendInteractive posts the m.room.message carrying the component blocks in
// unsigned.mxui, with an HTML fallback rendering for stock clients.
func (c *Client) SendInteractive(roomID string, p mxevent.MessagePayload) (string, error) {
	html := p.HTML
	if html == "" {
		html = escapeHTML(p.Body)
	}
	// Fallback markup appended for plain clients: mxui:// deep-links.
	fallback := `<hr><small>mxui interactive — ` + renderFallbackLinks(p.Blocks) + `</small>`
	content := map[string]any{
		"msgtype": "m.text",
		"body":    p.Body + "\n(mxui: " + fallbackLabels(p.Blocks) + ")",
		"format":  "org.matrix.custom.html",
		"formatted_body": html + fallback,
		"unsigned": map[string]any{"mxui": map[string]any{"blocks": p.Blocks}},
	}
	// NB: unsigned on send is not persisted by all HS; continuwuity keeps
	// client-supplied unsigned.mxui for same-server render. The canonical
	// location for v2 is a dedicated io.mxui.component_message event; MVP
	// optimizes for stock-client readability instead.
	var out struct {
		EventID string `json:"event_id"`
	}
	err := c.do("PUT", "/_matrix/client/v3/rooms/"+url.PathEscape(roomID)+"/send/m.room.message/"+
		fmt.Sprintf("mxui%d", time.Now().UnixNano()), content, &out)
	return out.EventID, err
}

// EditMessage replaces the body of a previous message (m.room.message edit).
func (c *Client) EditMessage(roomID, eventID, newBody string) error {
	content := map[string]any{
		"msgtype":       "m.text",
		"body":          "* " + newBody,
		"m.new_content": map[string]any{"msgtype": "m.text", "body": newBody},
		"m.relates_to": map[string]any{
			"rel_type": "m.replace", "event_id": eventID,
		},
	}
	return c.do("PUT", "/_matrix/client/v3/rooms/"+url.PathEscape(roomID)+"/send/m.room.message/"+
		fmt.Sprintf("edit%d", time.Now().UnixNano()), content, nil)
}

// Sync streams room events once (long-poll) and returns new events.
type SyncResp struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]struct {
			Timeline struct {
				Events []RawEvent `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	}
}

type RawEvent struct {
	EventID string          `json:"event_id"`
	Type    string          `json:"type"`
	Sender  string          `json:"sender"`
	Content json.RawMessage `json:"content"`
}

func (c *Client) Sync(timeout time.Duration) (*SyncResp, error) {
	path := fmt.Sprintf("/_matrix/client/v3/sync?timeout=%d", int(timeout.Seconds()*1000))
	if c.Since != "" {
		path += "&since=" + url.QueryEscape(c.Since)
	}
	var out SyncResp
	if err := c.do("GET", path, nil, &out); err != nil {
		return nil, err
	}
	c.Since = out.NextBatch
	return &out, nil
}

// SendAction emits an io.mxui.component_action into the room (client-side use,
// e.g. the demo web client acting for a user).
func (c *Client) SendAction(roomID string, a *mxevent.ActionEvent) (string, error) {
	a.TriggerID = mxevent.SignTrigger(a)
	var out struct {
		EventID string `json:"event_id"`
	}
	err := c.do("PUT", "/_matrix/client/v3/rooms/"+url.PathEscape(roomID)+"/send/io.mxui.component_action/"+
		fmt.Sprintf("a%d", time.Now().UnixNano()), a, &out)
	return out.EventID, err
}

func escapeHTML(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func renderFallbackLinks(blocks []mxevent.ComponentBlock) string {
	out := ""
	for _, b := range blocks {
		for _, comp := range b.Components {
			out += `<a href="mxui://action/${comp.ID}">[` + escapeHTML(comp.Label) + `]</a> `
		}
	}
	return out
}

func fallbackLabels(blocks []mxevent.ComponentBlock) string {
	out := ""
	for _, b := range blocks {
		for _, comp := range b.Components {
			out += "[" + comp.Label + "] "
		}
	}
	return out
}
