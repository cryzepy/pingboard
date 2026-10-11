package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	maxHistory      = 60
	maxMonitors     = 20
	minIntervalSec  = 10
	maxIntervalSec  = 3600
	defaultInterval = 30
	maxNameLen      = 60
)

var (
	ErrNotFound = errors.New("monitor tidak ditemukan")
	ErrLimit    = fmt.Errorf("batas maksimum %d monitor tercapai", maxMonitors)
)

// ValidationError dikembalikan ketika input dari pengguna tidak valid.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// Check adalah hasil satu kali pengecekan URL.
type Check struct {
	Time       time.Time `json:"time"`
	Up         bool      `json:"up"`
	StatusCode int       `json:"statusCode"`
	LatencyMs  int64     `json:"latencyMs"`
	Error      string    `json:"error,omitempty"`
}

// Monitor adalah tampilan (snapshot) sebuah monitor untuk dikirim ke klien.
type Monitor struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	IntervalSec  int       `json:"intervalSec"`
	CreatedAt    time.Time `json:"createdAt"`
	Status       string    `json:"status"` // pending | up | down
	UptimePct    float64   `json:"uptimePct"`
	AvgLatencyMs int64     `json:"avgLatencyMs"`
	LastCheck    *Check    `json:"lastCheck"`
	History      []Check   `json:"history"`
}

// CheckFunc melakukan satu kali pengecekan. Dibuat sebagai fungsi agar mudah
// diganti dengan versi palsu saat testing.
type CheckFunc func(ctx context.Context, rawURL string) Check

type entry struct {
	id          string
	name        string
	url         string
	intervalSec int
	createdAt   time.Time
	history     []Check
	cancel      context.CancelFunc
}

// snapshot harus dipanggil saat lock Store sedang dipegang.
func (e *entry) snapshot() Monitor {
	m := Monitor{
		ID:          e.id,
		Name:        e.name,
		URL:         e.url,
		IntervalSec: e.intervalSec,
		CreatedAt:   e.createdAt,
		Status:      "pending",
		History:     append([]Check{}, e.history...),
	}
	n := len(e.history)
	if n == 0 {
		return m
	}
	last := e.history[n-1]
	m.LastCheck = &last
	m.Status = "down"
	if last.Up {
		m.Status = "up"
	}
	up := 0
	var total int64
	for _, c := range e.history {
		if c.Up {
			up++
			total += c.LatencyMs
		}
	}
	m.UptimePct = math.Round(float64(up)/float64(n)*1000) / 10
	if up > 0 {
		m.AvgLatencyMs = total / int64(up)
	}
	return m
}

// Store menyimpan semua monitor di memori. Setiap monitor dijalankan oleh satu
// goroutine sendiri yang mengecek URL secara berkala.
type Store struct {
	mu     sync.RWMutex
	items  map[string]*entry
	order  []string
	check  CheckFunc
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewStore(parent context.Context, check CheckFunc) *Store {
	ctx, cancel := context.WithCancel(parent)
	return &Store{
		items:  make(map[string]*entry),
		check:  check,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Shutdown menghentikan semua goroutine monitor dan menunggu sampai selesai.
func (s *Store) Shutdown() {
	s.cancel()
	s.wg.Wait()
}

func (s *Store) Add(name, rawURL string, intervalSec int) (Monitor, error) {
	name, rawURL, intervalSec, err := validateInput(name, rawURL, intervalSec)
	if err != nil {
		return Monitor{}, err
	}
	id, err := newID()
	if err != nil {
		return Monitor{}, err
	}

	s.mu.Lock()
	if len(s.items) >= maxMonitors {
		s.mu.Unlock()
		return Monitor{}, ErrLimit
	}
	ctx, cancel := context.WithCancel(s.ctx)
	e := &entry{
		id:          id,
		name:        name,
		url:         rawURL,
		intervalSec: intervalSec,
		createdAt:   time.Now().UTC(),
		cancel:      cancel,
	}
	s.items[id] = e
	s.order = append(s.order, id)
	snap := e.snapshot()
	s.wg.Add(1)
	s.mu.Unlock()

	go s.run(ctx, e)
	return snap, nil
}

func (s *Store) List() []Monitor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Monitor, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.items[id].snapshot())
	}
	return out
}

func (s *Store) Get(id string) (Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.items[id]
	if !ok {
		return Monitor{}, ErrNotFound
	}
	return e.snapshot(), nil
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok {
		return ErrNotFound
	}
	e.cancel()
	delete(s.items, id)
	for i, v := range s.order {
		if v == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}

// CheckNow menjalankan pengecekan segera (tanpa menunggu interval).
func (s *Store) CheckNow(ctx context.Context, id string) (Monitor, error) {
	s.mu.RLock()
	e, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		return Monitor{}, ErrNotFound
	}
	s.record(e, s.check(ctx, e.url))
	return s.Get(id)
}

func (s *Store) run(ctx context.Context, e *entry) {
	defer s.wg.Done()
	s.probe(ctx, e)
	t := time.NewTicker(time.Duration(e.intervalSec) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.probe(ctx, e)
		}
	}
}

func (s *Store) probe(ctx context.Context, e *entry) {
	c := s.check(ctx, e.url)
	if ctx.Err() != nil {
		return // monitor dihapus atau server berhenti saat pengecekan berjalan
	}
	s.record(e, c)
}

func (s *Store) record(e *entry, c Check) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.history = append(e.history, c)
	if n := len(e.history); n > maxHistory {
		e.history = append([]Check(nil), e.history[n-maxHistory:]...)
	}
}

func validateInput(name, rawURL string, intervalSec int) (string, string, int, error) {
	name = strings.TrimSpace(name)
	rawURL = strings.TrimSpace(rawURL)

	if rawURL == "" {
		return "", "", 0, &ValidationError{"url wajib diisi"}
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", 0, &ValidationError{"url harus diawali http:// atau https:// dan memiliki host yang valid"}
	}
	if len(rawURL) > 2048 {
		return "", "", 0, &ValidationError{"url terlalu panjang"}
	}

	if name == "" {
		name = u.Hostname()
	}
	if len([]rune(name)) > maxNameLen {
		return "", "", 0, &ValidationError{fmt.Sprintf("nama maksimal %d karakter", maxNameLen)}
	}

	if intervalSec == 0 {
		intervalSec = defaultInterval
	}
	if intervalSec < minIntervalSec || intervalSec > maxIntervalSec {
		return "", "", 0, &ValidationError{
			fmt.Sprintf("interval harus antara %d dan %d detik", minIntervalSec, maxIntervalSec),
		}
	}
	return name, rawURL, intervalSec, nil
}

func newID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
