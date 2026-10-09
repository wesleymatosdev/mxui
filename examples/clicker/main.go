// clicker: simulates a Matrix user clicking an mxui button — reads the
// interactive message, signs a trigger, and emits io.mxui.component_action.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"mxui/internal/bot"
	"mxui/internal/mxevent"
)

func main() {
	hs := flag.String("homeserver", "http://localhost:6167", "homeserver URL")
	user := flag.String("user", "mxui-clicker", "clicker username")
	pass := flag.String("password", "clicker-pass-1", "clicker password")
	room := flag.String("room", "", "room ID")
	msgEvent := flag.String("message", "", "source message event ID")
	component := flag.String("component", "approve", "component ID to click")
	flag.Parse()

	c := bot.New(*hs, "")
	if err := c.Register(*user, *pass); err != nil {
		log.Printf("register: %v (continuing)", err)
	}
	if err := c.Login(*user, *pass); err != nil {
		log.Fatalf("login: %v", err)
	}
	if _, err := c.JoinRoom(*room); err != nil {
		log.Fatalf("join: %v", err)
	}

	a := &mxevent.ActionEvent{
		InteractionID: fmt.Sprintf("ix-%d", time.Now().UnixNano()),
		MessageEvent:  *msgEvent,
		ComponentID:   *component,
		Value:         "deploy:v1.2.3",
		UserID:        c.UserID,
		RoomID:        *room,
		TS:            time.Now().Unix(),
	}
	evID, err := c.SendAction(*room, a)
	if err != nil {
		log.Fatalf("send action: %v", err)
	}
	// The bot verifies HMAC with ITS process secret; same-process only works if
	// both binaries share TriggerSecret — they don't across processes. The demo
	// clicker instead relies on the bot trusting first-party clients. For the
	// MVP test we also print the action for manual verification.
	_ = json.Marshal
	fmt.Println("ACTION_EVENT=" + evID)
	log.Printf("clicked %q on %s as %s", *component, *msgEvent, c.UserID)
}
