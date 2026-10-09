package mxevent

import (
	"testing"
	"time"
)

func TestTriggerTokenDeterministic(t *testing.T) {
	a := TriggerToken([]byte("s"), "m1", "approve", "@u:a.b", 1000)
	b := TriggerToken([]byte("s"), "m1", "approve", "@u:a.b", 1000)
	if a != b {
		t.Fatal("same inputs must give same token")
	}
	for _, diff := range [][3]string{
		{"m2", "approve", "@u:a.b"},
		{"m1", "reject", "@u:a.b"},
		{"m1", "approve", "@u:c.d"},
	} {
		if c := TriggerToken([]byte("s"), diff[0], diff[1], diff[2], 1000); a == c {
			t.Fatalf("token collision for %v", diff)
		}
	}
	if d := TriggerToken([]byte("other"), "m1", "approve", "@u:a.b", 1000); a == d {
		t.Fatal("different secret must give different token")
	}
}

func TestDedupeSingleUse(t *testing.T) {
	RecordUse("m", "approve", "@u")
	if !IsUsed("m", "approve", "@u") {
		t.Fatal("recorded interaction must be used")
	}
	if IsUsed("m", "reject", "@u") || IsUsed("m", "approve", "@v") {
		t.Fatal("different interactions must not be blocked")
	}
	PruneDedupe(0) // prune everything older than now
	time.Sleep(10 * time.Millisecond)
	PruneDedupe(5 * time.Millisecond)
	if IsUsed("m", "approve", "@u") {
		t.Fatal("pruned entry must be gone")
	}
}

func TestStale(t *testing.T) {
	if !CheckStale(time.Now().Add(-time.Hour).Unix()) {
		t.Fatal("past expiry must be stale")
	}
	if CheckStale(time.Now().Add(time.Hour).Unix()) {
		t.Fatal("future expiry must not be stale")
	}
}

func TestLegacyTriggerV0(t *testing.T) {
	// v0 HMAC scheme still used by clicker example: sign+verify round trip,
	// TTL, replay rejection.
	old := TriggerSecret
	TriggerSecret = []byte("unit-secret")
	defer func() { TriggerSecret = old; RestoreTriggers() }()

	a := &ActionEvent{
		InteractionID: "ix-1", MessageEvent: "$m", ComponentID: "approve",
		UserID: "@u", RoomID: "!r", TS: time.Now().Unix(),
	}
	a.TriggerID = SignTrigger(a)
	if !VerifyTrigger(a) {
		t.Fatal("valid trigger must verify")
	}
	if VerifyTrigger(a) {
		t.Fatal("replayed trigger must be rejected")
	}
	RestoreTriggers()
	expired := &ActionEvent{
		InteractionID: "ix-2", MessageEvent: "$m", ComponentID: "approve",
		UserID: "@u", RoomID: "!r", TS: time.Now().Add(-2 * time.Minute).Unix(),
	}
	expired.TriggerID = SignTrigger(expired)
	RestoreTriggers()
	if VerifyTrigger(expired) {
		t.Fatal("expired trigger must be rejected")
	}
	forged := &ActionEvent{
		InteractionID: "ix-3", MessageEvent: "$m", ComponentID: "approve",
		UserID: "@u", RoomID: "!r", TS: time.Now().Unix(), TriggerID: "deadbeef",
	}
	if VerifyTrigger(forged) {
		t.Fatal("forged trigger must be rejected")
	}
}
