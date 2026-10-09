package mxevent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ComponentMessageEvent is the content of io.mxui.component_message — the
// persistent, client-readable definition of an interactive message. It rides
// the room DAG like any event, so a cold sync sees it without server state.
type ComponentMessageEvent struct {
	Blocks []ComponentBlock `json:"blocks"`
	Expiry int64            `json:"expiry"` // unix seconds; clicks after this are stale
}

// TriggerToken derives the per-component HMAC the bot verifies. Key = bot's
// private secret; inputs = message event id + component id + user + expiry.
// Unlike the v0 global-secret scheme, no secret is ever shared with clients:
// the client sends the action unsigned and the BOT authenticates by membership
// + freshness + single-use. (Spoofing by other room members is accepted in v1
// — same trust level as Discord component clicks — documented in README.)
func TriggerToken(secret []byte, messageEvent, componentID, userID string, expiry int64) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(messageEvent))
	mac.Write([]byte{0})
	mac.Write([]byte(componentID))
	mac.Write([]byte{0})
	mac.Write([]byte(userID))
	mac.Write([]byte{0})
	buf := []byte(fmt.Sprintf("%d", expiry))
	mac.Write(buf)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyAction validates a v1 action: message not expired, sender is a room
// member (checked by caller against the HS), interaction not yet used.
// Single-use is tracked per (message, component, user).
type dedupe struct{ used map[string]time.Time }

var dedupeStore = &dedupe{used: map[string]time.Time{}}

// RecordUse marks an interaction as consumed.
func RecordUse(messageEvent, componentID, userID string) {
	dedupeStore.used[messageEvent+"|"+componentID+"|"+userID] = time.Now()
}

// IsUsed reports whether this exact interaction already fired (idempotent UI).
func IsUsed(messageEvent, componentID, userID string) bool {
	_, ok := dedupeStore.used[messageEvent+"|"+componentID+"|"+userID]
	return ok
}

// Prune clears entries older than maxAge (run periodically by the bot).
func (d *dedupe) Prune(maxAge time.Duration) {
	cut := time.Now().Add(-maxAge)
	for k, t := range d.used {
		if t.Before(cut) {
			delete(d.used, k)
		}
	}
}

func PruneDedupe(maxAge time.Duration) { dedupeStore.Prune(maxAge) }

// CheckStale reports whether the component message has expired.
func CheckStale(expiry int64) bool { return time.Now().Unix() > expiry }
