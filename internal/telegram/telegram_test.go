package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessagePostsChatAndText(t *testing.T) {
	var gotChat, gotText, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseForm(); err != nil {
			t.Fatalf("form ayristirilamadi: %v", err)
		}
		gotChat = r.FormValue("chat_id")
		gotText = r.FormValue("text")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
	}))
	defer srv.Close()

	c := Client{Token: "TESTTOKEN", HTTPClient: srv.Client(), BaseURL: srv.URL}
	if err := c.SendMessage(42, "Kapı açıldı"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotChat != "42" || gotText != "Kapı açıldı" {
		t.Errorf("beklenmeyen istek: chat=%q text=%q", gotChat, gotText)
	}
	if !strings.HasSuffix(gotPath, "/sendMessage") {
		t.Errorf("beklenmeyen yol: %q", gotPath)
	}
}

func TestSendMessageReturnsErrorOnAPIFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "Forbidden: bot was blocked"})
	}))
	defer srv.Close()

	c := Client{Token: "t", HTTPClient: srv.Client(), BaseURL: srv.URL}
	err := c.SendMessage(1, "x")
	if err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Fatalf("beklenen hata gelmedi: %v", err)
	}
}

func TestGetUpdatesSendsOffsetAndTimeout(t *testing.T) {
	var gotOffset, gotTimeout string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOffset = r.URL.Query().Get("offset")
		gotTimeout = r.URL.Query().Get("timeout")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{}})
	}))
	defer srv.Close()

	c := Client{Token: "t", HTTPClient: srv.Client(), BaseURL: srv.URL}
	if _, err := c.GetUpdates(99, 25); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if gotOffset != "99" || gotTimeout != "25" {
		t.Errorf("beklenmeyen sorgu: offset=%q timeout=%q", gotOffset, gotTimeout)
	}
}

func TestGetUpdatesKeepsUpdateIDForNonMessageUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"result":[
			{"update_id":10,"message":{"chat":{"id":555},"text":"/durum"}},
			{"update_id":11}
		]}`))
	}))
	defer srv.Close()

	c := Client{Token: "t", HTTPClient: srv.Client(), BaseURL: srv.URL}
	updates, err := c.GetUpdates(0, 25)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("2 guncelleme bekleniyordu, %d geldi", len(updates))
	}
	if updates[0].UpdateID != 10 || updates[0].Chat != 555 || updates[0].Text != "/durum" {
		t.Errorf("ilk guncelleme yanlis ayristi: %+v", updates[0])
	}
	if updates[1].UpdateID != 11 || updates[1].Chat != 0 {
		t.Errorf("mesajsiz guncellemenin UpdateID'si korunmali, Chat sifir olmali: %+v", updates[1])
	}
}
