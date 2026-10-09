// approval-bot: demo of the mxui loop — posts "Deploy v1.2.3? [Approve] [Reject]",
// listens for io.mxui.component_action, edits the message with the outcome.
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
	user := flag.String("user", "mxui-bot", "bot username")
	pass := flag.String("password", "mxui-dev-pass", "bot password")
	room := flag.String("room", "", "room alias or ID to join")
	flag.Parse()

	c := bot.New(*hs, "")
	if err := c.Register(*user, *pass); err != nil {
		// already registered — fall through to login
	}
	if err := c.Login(*user, *pass); err != nil {
		log.Fatalf("login: %v", err)
	}
	log.Printf("bot online as %s", c.UserID)

	roomID := *room
	if roomID == "" {
		var err error
		roomID, err = c.CreateRoom("mxui-demo")
		if err != nil {
			log.Fatalf("create room: %v", err)
		}
		log.Printf("created/joined room %s", roomID)
	} else {
		var err error
		roomID, err = c.JoinRoom(roomID)
		if err != nil {
			log.Fatalf("join room: %v", err)
		}
	}
	fmt.Println("ROOM=" + roomID)

	eventID, err := c.SendInteractive(roomID, mxevent.MessagePayload{
		Body: "Deploy v1.2.3 to production?",
		Blocks: []mxevent.ComponentBlock{{
			BlockID: "deploy",
			Components: []mxevent.Component{
				{ID: "approve", Type: "button", Label: "Approve", Style: "primary", Value: "deploy:v1.2.3"},
				{ID: "reject", Type: "button", Label: "Reject", Style: "danger", Value: "deploy:v1.2.3"},
			},
		}},
	})
	if err != nil {
		log.Fatalf("send interactive: %v", err)
	}
	fmt.Println("MESSAGE=" + eventID)
	log.Printf("interactive message sent (%s); waiting for clicks...", eventID)

	for {
		sync, err := c.Sync(30 * time.Second)
		if err != nil {
			log.Printf("sync: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for rid, r := range sync.Rooms.Join {
			for _, ev := range r.Timeline.Events {
				if ev.Type != "io.mxui.component_action" || ev.Sender == c.UserID {
					continue
				}
				var a mxevent.ActionEvent
				if err := json.Unmarshal(ev.Content, &a); err != nil {
					log.Printf("bad action event: %v", err)
					continue
				}
				if !mxevent.VerifyTrigger(&a) {
					log.Printf("REJECTED forged/expired trigger from %s", a.UserID)
					// still acknowledge visibly, mimicking Discord's silent drop
					continue
				}
				log.Printf("interaction: %s clicked %q (value=%s)", a.UserID, a.ComponentID, a.Value)
				_ = rid
				outcome := "✅ Approved by " + a.UserID
				if a.ComponentID == "reject" {
					outcome = "❌ Rejected by " + a.UserID
				}
				if err := c.EditMessage(roomID, eventID, "Deploy v1.2.3 to production? — "+outcome); err != nil {
					log.Printf("edit: %v", err)
				} else {
					log.Printf("message edited: %s", outcome)
				}
			}
		}
	}
}
