// Package mxevent defines the mxui interaction event layer over Matrix.
//
// The core object is a ComponentMessage: an m.room.message carrying structured
// component definitions in unsigned (or content, for encrypted rooms v2), plus
// the io.mxui.component_action event a client emits when a user clicks.
package mxevent

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"
)

// Component types v1. Buttons only — the minimal loop to prove.
const (
	TypeButton = "button"
)

// Component is one interactive element attached to a message.
type Component struct {
	ID    string `json:"id"`              // actionId, unique per message
	Type  string `json:"type"`            // "button"
	Label string `json:"label"`           // visible text
	Style string `json:"style,omitempty"` // primary | danger | default
	Value string `json:"value,omitempty"` // opaque payload to the bot
}

// ComponentBlock groups components (UIKit-style actions block).
type ComponentBlock struct {
	BlockID    string      `json:"blockId"`
	Components []Component `json:"components"`
}

// MessagePayload is what a bot sends to mxui-server to post an interactive message.
type MessagePayload struct {
	RoomID  string          `json:"roomId"`
	Body    string          `json:"body"`    // plain-text body / fallback
	HTML    string          `json:"html,omitempty"`
	Blocks  []ComponentBlock `json:"blocks,omitempty"`
}

// ActionEvent is the content of io.mxui.component_action — what a client sends
// when the user activates a component. Carries UIKit triggerId semantics:
// single-use, HMAC-signed, short TTL, proving the click came from the rendered UI.
type ActionEvent struct {
	InteractionID string    `json:"interactionId"` // server-issued, unique per message+component
	MessageEvent  string    `json:"messageEvent"`  // event ID of the source message
	ComponentID   string    `json:"componentId"`
	Value         string    `json:"value,omitempty"`
	UserID        string    `json:"userId"`
	RoomID        string    `json:"roomId"`
	TS            int64     `json:"ts"`
	TriggerID     string    `json:"triggerId"` // HMAC over the fields above + nonce
}

// TriggerSecret is the HMAC key shared between mxui-server and clients.
// In production: per-room secret delivered to clients at render time via
// unsigned room state or an encrypted to-device message. For the MVP a single
// process-wide secret is fine; MXUI_TRIGGER_SECRET lets multiple processes
// (bot + demo clicker) share one during development.
var TriggerSecret []byte

func init() {
	if env := os.Getenv("MXUI_TRIGGER_SECRET"); env != "" {
		TriggerSecret = []byte(env)
		return
	}
	TriggerSecret = make([]byte, 32)
	rand.Read(TriggerSecret)
}

// SignTrigger computes the single-use trigger token for an action.
func SignTrigger(a *ActionEvent) string {
	mac := hmac.New(sha256.New, TriggerSecret)
	mac.Write([]byte(a.InteractionID))
	mac.Write([]byte(a.MessageEvent))
	mac.Write([]byte(a.ComponentID))
	mac.Write([]byte(a.UserID))
	mac.Write([]byte(a.RoomID))
	buf, _ := json.Marshal(a.TS)
	mac.Write(buf)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyTrigger checks signature, single-use (nonce registry), and 60s TTL.
type triggerRegistry struct {
	used map[string]bool
}

var triggers = &triggerRegistry{used: map[string]bool{}}

func VerifyTrigger(a *ActionEvent) bool {
	if a.TS < time.Now().Add(-60*time.Second).Unix() {
		return false // expired
	}
	expect := SignTrigger(a)
	if !hmac.Equal([]byte(expect), []byte(a.TriggerID)) {
		return false // forged
	}
	key := a.InteractionID + "|" + a.TriggerID
	if triggers.used[key] {
		return false // replayed
	}
	triggers.used[key] = true
	return true
}

// InteractionCallback is what mxui-server POSTs to the bot's webhook URL.
type InteractionCallback struct {
	Action ActionEvent        `json:"action"`
	State  map[string]string  `json:"state,omitempty"` // componentId -> value snapshot
}
