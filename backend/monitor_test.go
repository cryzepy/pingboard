package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeCheck(up bool) CheckFunc {
	return func(ctx context.Context, _ string) Check {
		c := Check{Time: time.Now().UTC(), Up: up, LatencyMs: 42}
		if up {
			c.StatusCode = 200
		} else {
			c.StatusCode = 503
			c.Error = "HTTP 503"
		}
		return c
	}
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("kondisi tidak terpenuhi dalam 2 detik")
}

func newTestStore(t *testing.T, up bool) *Store {
	t.Helper()
	s := NewStore(context.Background(), fakeCheck(up))
	t.Cleanup(s.Shutdown)
	return s
}

func TestValidateInput(t *testing.T) {
	tests := []struct {
		name     string
		in       [3]any
		wantErr  bool
		wantName string
		wantIvl  int
	}{
		{"valid", [3]any{"Situs", "https://example.com", 30}, false, "Situs", 30},
		{"nama default dari host", [3]any{"", "https://example.com/path", 0}, false, "example.com", 30},
		{"url kosong", [3]any{"x", "", 30}, true, "", 0},
		{"skema ftp", [3]any{"x", "ftp://example.com", 30}, true, "", 0},
		{"skema javascript", [3]any{"x", "javascript:alert(1)", 30}, true, "", 0},
		{"tanpa host", [3]any{"x", "http://", 30}, true, "", 0},
		{"interval terlalu kecil", [3]any{"x", "https://example.com", 5}, true, "", 0},
		{"interval terlalu besar", [3]any{"x", "https://example.com", 99999}, true, "", 0},
		{"nama terlalu panjang", [3]any{strings.Repeat("a", 61), "https://example.com", 30}, true, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, _, ivl, err := validateInput(tt.in[0].(string), tt.in[1].(string), tt.in[2].(int))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if err == nil && (name != tt.wantName || ivl != tt.wantIvl) {
				t.Fatalf("got (%q, %d), want (%q, %d)", name, ivl, tt.wantName, tt.wantIvl)
			}
		})
	}
}

func TestAddRunsImmediateCheck(t *testing.T) {
	s := newTestStore(t, true)
	m, err := s.Add("Test", "https://example.com", 30)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "pending" {
		t.Fatalf("status awal = %q, want pending", m.Status)
	}
	eventually(t, func() bool {
		got, _ := s.Get(m.ID)
		return got.Status == "up" && len(got.History) == 1
	})
}

func TestDownStatus(t *testing.T) {
	s := newTestStore(t, false)
	m, _ := s.Add("Mati", "https://example.com", 30)
	eventually(t, func() bool {
		got, _ := s.Get(m.ID)
		return got.Status == "down" && got.UptimePct == 0
	})
}

func TestLimit(t *testing.T) {
	s := newTestStore(t, true)
	for i := 0; i < maxMonitors; i++ {
		if _, err := s.Add("m", "https://example.com", 30); err != nil {
			t.Fatalf("add #%d: %v", i, err)
		}
	}
	if _, err := s.Add("m", "https://example.com", 30); err != ErrLimit {
		t.Fatalf("err = %v, want ErrLimit", err)
	}
}

func TestRemove(t *testing.T) {
	s := newTestStore(t, true)
	m, _ := s.Add("m", "https://example.com", 30)
	if err := s.Remove(m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(m.ID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := s.Remove(m.ID); err != ErrNotFound {
		t.Fatalf("hapus dua kali: err = %v, want ErrNotFound", err)
	}
	if len(s.List()) != 0 {
		t.Fatal("list harus kosong")
	}
}

func TestSnapshotStats(t *testing.T) {
	e := &entry{id: "x", history: []Check{
		{Up: true, LatencyMs: 10},
		{Up: true, LatencyMs: 20},
		{Up: true, LatencyMs: 30},
		{Up: false, LatencyMs: 999},
	}}
	m := e.snapshot()
	if m.UptimePct != 75 {
		t.Errorf("uptime = %v, want 75", m.UptimePct)
	}
	if m.AvgLatencyMs != 20 {
		t.Errorf("avg latency = %d, want 20 (hanya hitung check yang up)", m.AvgLatencyMs)
	}
	if m.Status != "down" {
		t.Errorf("status = %q, want down", m.Status)
	}
}

func TestHistoryIsCapped(t *testing.T) {
	s := newTestStore(t, true)
	e := &entry{id: "x"}
	for i := 0; i < maxHistory+40; i++ {
		s.record(e, Check{Up: true, LatencyMs: int64(i)})
	}
	if len(e.history) != maxHistory {
		t.Fatalf("len = %d, want %d", len(e.history), maxHistory)
	}
	if e.history[len(e.history)-1].LatencyMs != int64(maxHistory+39) {
		t.Fatal("data terbaru harus tetap ada")
	}
}

func TestAPIFlow(t *testing.T) {
	s := newTestStore(t, true)
	srv := httptest.NewServer((&API{store: s}).Routes())
	defer srv.Close()

	do := func(method, path, body string) (*http.Response, []byte) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, bytes.NewBufferString(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return resp, buf.Bytes()
	}

	// buat
	resp, body := do("POST", "/api/monitors", `{"name":"Demo","url":"https://example.com","intervalSec":30}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var created Monitor
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		t.Fatalf("respons create tidak valid: %v %s", err, body)
	}

	// list
	resp, body = do("GET", "/api/monitors", "")
	var list []Monitor
	json.Unmarshal(body, &list)
	if resp.StatusCode != 200 || len(list) != 1 {
		t.Fatalf("list: %d len=%d", resp.StatusCode, len(list))
	}

	// check now
	resp, body = do("POST", "/api/monitors/"+created.ID+"/check", "")
	if resp.StatusCode != 200 {
		t.Fatalf("check: %d %s", resp.StatusCode, body)
	}
	var checked Monitor
	json.Unmarshal(body, &checked)
	if checked.Status != "up" || len(checked.History) == 0 {
		t.Fatalf("hasil check tidak sesuai: %+v", checked)
	}

	// input tidak valid
	for _, bad := range []string{
		`{"url":"ftp://x.com"}`,
		`{"url":"https://x.com","bogus":1}`,
		`bukan json`,
	} {
		resp, _ = do("POST", "/api/monitors", bad)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("body %q: status %d, want 400", bad, resp.StatusCode)
		}
	}

	// hapus
	resp, _ = do("DELETE", "/api/monitors/"+created.ID, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp, _ = do("GET", "/api/monitors/"+created.ID, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get setelah delete: %d, want 404", resp.StatusCode)
	}
}

func TestCheckerBlocksPrivateAddresses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	blocked := NewHTTPChecker(false)(context.Background(), ts.URL)
	if blocked.Up {
		t.Fatal("alamat loopback seharusnya diblokir (proteksi SSRF)")
	}

	allowed := NewHTTPChecker(true)(context.Background(), ts.URL)
	if !allowed.Up || allowed.StatusCode != 200 {
		t.Fatalf("dengan allowPrivate harus up, got %+v", allowed)
	}
}

func TestCheckerStatusCodes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	check := NewHTTPChecker(true)
	if c := check(context.Background(), ts.URL+"/ok"); !c.Up || c.StatusCode != 204 {
		t.Errorf("204 harus up: %+v", c)
	}
	c := check(context.Background(), ts.URL+"/error")
	if c.Up || c.StatusCode != 500 || c.Error != "HTTP 500" {
		t.Errorf("500 harus down: %+v", c)
	}
}
