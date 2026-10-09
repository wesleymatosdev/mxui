// demo-bot: exercises the full v0.2 surface — buttons, select menus, modals,
// slash commands. Posts an interactive deploy card; "Configure" opens a modal;
// /deploy and /help commands respond ephemerally.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"mxui/internal/bot"
	"mxui/internal/mxevent"
)

func main() {
	hs := flag.String("homeserver", "http://localhost:6167", "homeserver URL")
	user := flag.String("user", "mxui-bot", "bot username")
	pass := flag.String("password", "mxui-dev-pass", "bot password")
	room := flag.String("room", "#mxui-demo:macbook.tailfddf66.ts.net", "room alias or ID")
	flag.Parse()

	c := bot.New(*hs, "")
	if err := c.Register(*user, *pass); err != nil {
		log.Printf("register: %v (continuing)", err)
	}
	if err := c.Login(*user, *pass); err != nil {
		log.Fatalf("login: %v", err)
	}
	log.Printf("bot online as %s", c.UserID)

	roomID, err := c.JoinRoom(*room)
	if err != nil {
		log.Fatalf("join room: %v", err)
	}
	fmt.Println("ROOM=" + roomID)

	eventID, err := c.SendInteractive(roomID, mxevent.MessagePayload{
		Body: "Deploy to production?",
		Blocks: []mxevent.ComponentBlock{{
			BlockID: "deploy",
			Components: []mxevent.Component{
				{ID: "approve", Type: "button", Label: "Approve", Style: "primary", Value: "deploy"},
				{ID: "reject", Type: "button", Label: "Reject", Style: "danger", Value: "deploy"},
				{ID: "env", Type: "select", Label: "Environment", Style: "default", Value: "",
					Options: []mxevent.SelectOption{
						{Label: "Production", Value: "prod"},
						{Label: "Staging", Value: "staging"},
						{Label: "Canary", Value: "canary"},
					}},
				{ID: "configure", Type: "button", Label: "Configure…", Style: "default", Value: "config"},
			},
		}},
	})
	if err != nil {
		log.Fatalf("send interactive: %v", err)
	}
	fmt.Println("MESSAGE=" + eventID)
	log.Printf("interactive card sent (%s)", eventID)

	expiryByMsg := map[string]int64{}

	for {
		sync, err := c.Sync(30 * time.Second)
		if err != nil {
			log.Printf("sync: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for rid, r := range sync.Rooms.Join {
			for _, ev := range r.Timeline.Events {
				if ev.Type == "io.mxui.component_message" {
					var cme struct {
						Expiry    int64 `json:"expiry"`
						RelatesTo struct {
							EventID string `json:"event_id"`
						} `json:"m.relates_to"`
					}
					if err := json.Unmarshal(ev.Content, &cme); err == nil && cme.RelatesTo.EventID != "" {
						expiryByMsg[cme.RelatesTo.EventID] = cme.Expiry
					}
				}
			}
			for _, ev := range r.Timeline.Events {
				if ev.Sender == c.UserID {
					continue
				}
				_ = rid
				switch ev.Type {
				case "io.mxui.component_action":
					var a mxevent.ActionEvent
					if err := json.Unmarshal(ev.Content, &a); err != nil {
						continue
					}
					fresh := time.Now().Unix()-a.TS < 120 && a.TS <= time.Now().Unix()+120
					notStale := true
					if exp, found := expiryByMsg[a.MessageEvent]; found {
						notStale = !mxevent.CheckStale(exp)
					}
					if !fresh || !notStale || mxevent.IsUsed(a.MessageEvent, a.ComponentID, a.UserID) {
						log.Printf("REJECTED action from %s (component=%s)", a.UserID, a.ComponentID)
						continue
					}
					mxevent.RecordUse(a.MessageEvent, a.ComponentID, a.UserID)
					log.Printf("interaction: %s → %s (value=%s)", a.UserID, a.ComponentID, a.Value)
					switch a.ComponentID {
					case "approve":
						env := envOf(a.Value)
						c.EditMessage(roomID, eventID, "Deploy to production? — ✅ Approved by "+a.UserID+" (env: "+env+")")
					case "reject":
						c.EditMessage(roomID, eventID, "Deploy to production? — ❌ Rejected by "+a.UserID)
					case "configure":
						c.PresentModal(roomID, a, map[string]any{
							"title": "Configure deployment",
							"fields": []map[string]any{
								{"id": "version", "label": "Version tag", "type": "text", "placeholder": "v1.2.3"},
								{"id": "replicas", "label": "Replicas", "type": "number", "placeholder": "3"},
								{"id": "strategy", "label": "Strategy", "type": "select", "options": []map[string]string{
									{"label": "Rolling", "value": "rolling"},
									{"label": "Blue/Green", "value": "bluegreen"},
								}},
							},
						})
					}
				case "io.mxui.modal_submit":
					var m struct {
						ModalEvent string         `json:"modalEvent"`
						Values     map[string]string `json:"values"`
						UserID     string         `json:"userId"`
					}
					if err := json.Unmarshal(ev.Content, &m); err != nil {
						continue
					}
					log.Printf("modal submit from %s: %v", m.UserID, m.Values)
					c.EditMessage(roomID, eventID, fmt.Sprintf(
						"Deploy to production? — 🚀 Scheduled by %s: %s ×%s replicas (%s)",
						m.UserID, m.Values["version"], m.Values["replicas"], m.Values["strategy"]))
				case "io.mxui.command":
					var cmd struct {
						Command string `json:"command"`
						Args    string `json:"args"`
						UserID  string `json:"userId"`
					}
					if err := json.Unmarshal(ev.Content, &cmd); err != nil {
						continue
					}
					log.Printf("command /%s %s from %s", cmd.Command, cmd.Args, cmd.UserID)
					switch cmd.Command {
					case "deploy":
						v := strings.TrimSpace(cmd.Args)
						if v == "" {
							v = "v1.2.3"
						}
						c.SendEphemeral(roomID, cmd.UserID, "🚀 Deploying "+v+" — use the card above to approve.")
					case "card":
						eventID, err = c.SendInteractive(roomID, mxevent.MessagePayload{
							Body: "Deploy to production? (fresh card)",
							Blocks: []mxevent.ComponentBlock{{
								BlockID: "deploy",
								Components: []mxevent.Component{
									{ID: "approve", Type: "button", Label: "Approve", Style: "primary", Value: "deploy"},
									{ID: "reject", Type: "button", Label: "Reject", Style: "danger", Value: "deploy"},
									{ID: "env", Type: "select", Label: "Environment", Value: "",
										Options: []mxevent.SelectOption{
											{Label: "Production", Value: "prod"},
											{Label: "Staging", Value: "staging"},
											{Label: "Canary", Value: "canary"},
										}},
									{ID: "configure", Type: "button", Label: "Configure…", Value: "config"},
								},
							}},
						})
						if err == nil {
							log.Printf("fresh card sent (%s)", eventID)
						}
					case "help":
						c.SendEphemeral(roomID, cmd.UserID, "commands: /deploy <version>, /card (fresh interactive card), /help.")
					default:
						c.SendEphemeral(roomID, cmd.UserID, "unknown command /"+cmd.Command)
					}
				}
			}
		}
	}
}

func envOf(v string) string {
	switch v {
	case "staging", "canary":
		return v
	default:
		return "prod"
	}
}
