// webhook-bot: an mxui bot with NO Matrix code whatsoever. It registers with
// mxui-server, receives interactions on an HTTP webhook, and responds via the
// REST API. This is the v0.4.0 acceptance target.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

var (
	apiBase = envOr("MXUI_API", "http://127.0.0.1:8790")
	token   string
	msgID   string
	mu      sync.Mutex
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func api(kind string, payload any, out any) error {
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", apiBase+"/api/v1/messages", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s: %s: %s", kind, resp.Status, body)
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

func main() {
	// register with the gateway
	reg, _ := json.Marshal(map[string]string{
		"name": "webhook-deploy-bot",
		"webhookUrl": "http://127.0.0.1:8792/interactions",
	})
	resp, err := http.Post(apiBase+"/api/v1/bots", "application/json", bytes.NewReader(reg))
	if err != nil {
		log.Fatalf("register: %v", err)
	}
	var rr struct{ Token string `json:"token"` }
	json.NewDecoder(resp.Body).Decode(&rr)
	resp.Body.Close()
	token = rr.Token
	if token == "" {
		log.Fatal("no token from gateway")
	}
	fmt.Println("WEBHOOK_BOT_REGISTERED token=" + token[:12] + "…")

	// post the interactive card via REST
	var sr struct{ MessageID string `json:"messageId"` }
	err = api("interactive", map[string]any{
		"kind": "interactive",
		"body": "Webhook-bot deploy card",
		"blocks": []map[string]any{{
			"blockId": "deploy",
			"components": []map[string]any{
				{"id": "approve", "type": "button", "label": "Approve", "style": "primary", "value": "deploy"},
				{"id": "reject", "type": "button", "label": "Reject", "style": "danger", "value": "deploy"},
			},
		}},
	}, &sr)
	if err != nil {
		log.Fatalf("send: %v", err)
	}
	msgID = sr.MessageID
	fmt.Println("CARD_POSTED message=" + msgID)

	// webhook receiver
	http.HandleFunc("/interactions", func(w http.ResponseWriter, r *http.Request) {
		var ev struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch ev.Type {
		case "component_action":
			var a struct {
				ComponentID string `json:"componentId"`
				UserID      string `json:"userId"`
				Value       string `json:"value"`
			}
			json.Unmarshal(ev.Data, &a)
			log.Printf("webhook received: %s clicked %s", a.UserID, a.ComponentID)
			outcome := "✅ Approved by " + a.UserID
			if a.ComponentID == "reject" {
				outcome = "❌ Rejected by " + a.UserID
			}
			if err := api("edit", map[string]any{"kind": "edit", "newBody": "Webhook-bot deploy card — " + outcome}, nil); err != nil {
				log.Printf("edit: %v", err)
			} else {
				fmt.Println("CARD_EDITED " + outcome)
			}
		case "command":
			var cm struct {
				Command string `json:"command"`
				Args    string `json:"args"`
				UserID  string `json:"userId"`
			}
			json.Unmarshal(ev.Data, &cm)
			log.Printf("webhook command: /%s %s", cm.Command, cm.Args)
			api("ephemeral", map[string]any{"kind": "ephemeral", "userId": cm.UserID,
				"body": "webhook-bot says: /" + cm.Command + " acknowledged"}, nil)
		}
		w.WriteHeader(200)
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:8792", nil))
}
