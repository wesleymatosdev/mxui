// mxui-server: standalone gateway that lets bots use interactive Matrix
// without any Matrix code. Bots register (token + webhook URL); they POST
// JSON to send interactive messages / modals / ephemerals; mxui-server
// forwards user interactions to their webhook as Discord-shaped JSON.
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"mxui/internal/bot"
	"mxui/internal/mxevent"
)

type server struct {
	mu       sync.Mutex
	db       *sql.DB
	matrix   *bot.Client
	roomID   string
	msgByBot map[string]string // botToken -> last interactive message event id
}

// --- bot registration API ---

type registerReq struct {
	Name      string `json:"name"`
	WebhookURL string `json:"webhookUrl"`
}

type sendReq struct {
	Kind   string             `json:"kind"` // "interactive" | "modal" | "ephemeral" | "edit"
	Body   string             `json:"body"`
	Blocks []mxevent.ComponentBlock `json:"blocks,omitempty"`
	Modal  struct {
		Title  string           `json:"title"`
		Fields []map[string]any `json:"fields"`
	} `json:"modal"`
	Action mxevent.ActionEvent `json:"action"`
	NewBody string             `json:"newBody"`
	UserID  string             `json:"userId"`
}

func (s *server) handleSend(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var row struct {
		Name string
	}
	if err := s.db.QueryRow(`SELECT name FROM bots WHERE token = ?`, tok).
		Scan(&row.Name); err != nil {
		httpError(w, 401, "unknown bot token")
		return
	}
	var req sendReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, "bad json: "+err.Error())
		return
	}
	switch req.Kind {
	case "interactive":
		id, err := s.matrix.SendInteractive(s.roomID, mxevent.MessagePayload{
			Body: req.Body, Blocks: req.Blocks,
		})
		if err != nil {
			httpError(w, 502, err.Error())
			return
		}
		s.mu.Lock()
		s.msgByBot[tok] = id
		s.mu.Unlock()
		writeJSON(w, map[string]any{"messageId": id})
	case "modal":
		if err := s.matrix.PresentModal(s.roomID, req.Action, map[string]any{
			"title": req.Modal.Title, "fields": req.Modal.Fields,
		}); err != nil {
			httpError(w, 502, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case "ephemeral":
		if err := s.matrix.SendEphemeral(s.roomID, req.UserID, req.Body); err != nil {
			httpError(w, 502, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case "edit":
		s.mu.Lock()
		id := s.msgByBot[tok]
		s.mu.Unlock()
		if id == "" {
			httpError(w, 409, "no interactive message to edit")
			return
		}
		if err := s.matrix.EditMessage(s.roomID, id, req.NewBody); err != nil {
			httpError(w, 502, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		httpError(w, 400, "unknown kind")
	}
}

// --- interaction forwarding ---

func (s *server) forward(evType string, payload any) {
	s.mu.Lock()
	rows, err := s.db.Query(`SELECT webhook_url FROM bots`)
	s.mu.Unlock()
	if err != nil {
		log.Printf("webhook lookup: %v", err)
		return
	}
	var urls []string
	for rows.Next() {
		var u string
		if rows.Scan(&u) == nil {
			urls = append(urls, u)
		}
	}
	rows.Close()
	b, _ := json.Marshal(map[string]any{"type": evType, "data": payload})
	for _, u := range urls {
		go func(u string) {
			resp, err := http.Post(u, "application/json", bytes.NewReader(b))
			if err != nil {
				log.Printf("webhook %s: %v", u, err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}(u)
	}
}

func (s *server) runMatrixSync() {
	expiryByMsg := map[string]int64{}
	for {
		sync, err := s.matrix.Sync(30 * time.Second)
		if err != nil {
			log.Printf("sync: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, r := range sync.Rooms.Join {
			for _, ev := range r.Timeline.Events {
				if ev.Type == "io.mxui.component_message" {
					var cme struct {
						Expiry    int64 `json:"expiry"`
						RelatesTo struct {
							EventID string `json:"event_id"`
						} `json:"m.relates_to"`
					}
					if json.Unmarshal(ev.Content, &cme) == nil && cme.RelatesTo.EventID != "" {
						expiryByMsg[cme.RelatesTo.EventID] = cme.Expiry
					}
				}
			}
			for _, ev := range r.Timeline.Events {
				if ev.Sender == s.matrix.UserID {
					continue
				}
				switch ev.Type {
				case "io.mxui.component_action":
					var a mxevent.ActionEvent
					if json.Unmarshal(ev.Content, &a) != nil {
						continue
					}
					fresh := time.Now().Unix()-a.TS < 120 && a.TS <= time.Now().Unix()+120
					notStale := true
					if exp, found := expiryByMsg[a.MessageEvent]; found {
						notStale = !mxevent.CheckStale(exp)
					}
					if !fresh || !notStale || mxevent.IsUsed(a.MessageEvent, a.ComponentID, a.UserID) {
						log.Printf("server REJECTED action from %s (%s)", a.UserID, a.ComponentID)
						continue
					}
					mxevent.RecordUse(a.MessageEvent, a.ComponentID, a.UserID)
					s.forward("component_action", a)
				case "io.mxui.modal_submit":
					var m map[string]any
					if json.Unmarshal(ev.Content, &m) == nil {
						s.forward("modal_submit", m)
					}
				case "io.mxui.command":
					var cm map[string]any
					if json.Unmarshal(ev.Content, &cm) == nil {
						s.forward("command", cm)
					}
				}
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func main() {
	hs := flag.String("homeserver", "http://localhost:6167", "")
	user := flag.String("user", "mxui-gw", "gateway matrix user")
	pass := flag.String("password", "mxui-gw-pass", "")
	room := flag.String("room", "#mxui-demo:macbook.tailfddf66.ts.net", "")
	dbPath := flag.String("db", "", "sqlite path (default mxui-server.db)")
	listen := flag.String("listen", "127.0.0.1:8790", "")
	flag.Parse()

	if *dbPath == "" {
		*dbPath = "mxui-server.db"
	}
	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS bots (
		token TEXT PRIMARY KEY, name TEXT NOT NULL, webhook_url TEXT NOT NULL,
		created_at INTEGER NOT NULL)`); err != nil {
		log.Fatalf("db schema: %v", err)
	}

	c := bot.New(*hs, "")
	if err := c.Register(*user, *pass); err != nil {
		log.Printf("register: %v (continuing)", err)
	}
	if err := c.Login(*user, *pass); err != nil {
		log.Fatalf("login: %v", err)
	}
	roomID, err := c.JoinRoom(*room)
	if err != nil {
		log.Fatalf("join: %v", err)
	}
	log.Printf("mxui-server online as %s in %s; listening on %s", c.UserID, roomID, *listen)

	s := &server{db: db, matrix: c, roomID: roomID, msgByBot: map[string]string{}}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/messages", s.handleSend)
	mux.HandleFunc("POST /api/v1/bots", func(w http.ResponseWriter, r *http.Request) {
		var rr registerReq
		if err := json.NewDecoder(r.Body).Decode(&rr); err != nil || rr.WebhookURL == "" {
			httpError(w, 400, "bad json or missing webhookUrl")
			return
		}
		tok := fmt.Sprintf("mxui_%d", time.Now().UnixNano())
		if _, err := db.Exec(`INSERT INTO bots (token,name,webhook_url,created_at)
			VALUES (?,?,?,?)`, tok, rr.Name, rr.WebhookURL, time.Now().Unix()); err != nil {
			httpError(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]any{"token": tok})
	})

	go s.runMatrixSync()
	log.Fatal(http.ListenAndServe(*listen, mux))
}
