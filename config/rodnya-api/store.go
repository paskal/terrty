package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	log "github.com/go-pkgz/lgr"
)

// limits that keep one page's log bounded even if its address leaks
const (
	maxUsersPerSet  = 1000
	maxEntriesUser  = 5000
	maxSetFileBytes = 64 << 20
)

var (
	errTooManyUsers   = errors.New("too many users on this page")
	errTooManyEntries = errors.New("too many entries for this user")
	errSetFull        = errors.New("page storage is full")
)

// record is one line of a page's append-only log; the state is the fold of all lines in order
type record struct {
	TS   time.Time       `json:"ts"`
	User string          `json:"user"`
	K    string          `json:"k,omitempty"`
	V    json.RawMessage `json:"v,omitempty"`
	Text *string         `json:"text,omitempty"`
}

// user is the folded state of one browser on one page
type user struct {
	ID      string                     `json:"id"`
	Updated time.Time                  `json:"updated"`
	Text    string                     `json:"text"`
	Entries map[string]json.RawMessage `json:"entries"`
}

type pageState struct {
	users map[string]*user
	file  *os.File
	size  int64
}

// store keeps every page's state in memory and appends each change to <dir>/<page>.jsonl
type store struct {
	dir   string
	limit int64 // log size that triggers compaction, maxSetFileBytes outside tests
	mu    sync.Mutex
	pages map[string]*pageState
}

func newStore(dir string) *store {
	return &store{dir: dir, limit: maxSetFileBytes, pages: map[string]*pageState{}}
}

func (s *store) path(page string) string { return filepath.Join(s.dir, page+".jsonl") }

// page returns the loaded state of a page, reading its log on first use; caller holds s.mu
func (s *store) page(page string) (*pageState, error) {
	if p, ok := s.pages[page]; ok {
		return p, nil
	}
	f, err := os.OpenFile(s.path(page), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	p := &pageState{users: map[string]*user{}, file: f}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	line := 0
	for sc.Scan() {
		line++
		p.size += int64(len(sc.Bytes())) + 1
		var r record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			log.Printf("[WARN] skip bad line %d in %s: %v", line, s.path(page), err)
			continue
		}
		p.apply(r)
	}
	if err := sc.Err(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("read log: %w", err)
	}
	s.pages[page] = p
	return p, nil
}

func (p *pageState) apply(r record) {
	u, ok := p.users[r.User]
	if !ok {
		u = &user{ID: r.User, Entries: map[string]json.RawMessage{}}
		p.users[r.User] = u
	}
	if r.TS.After(u.Updated) {
		u.Updated = r.TS
	}
	if r.K != "" {
		u.Entries[r.K] = r.V
	}
	if r.Text != nil {
		u.Text = *r.Text
	}
}

// changes drops records that would not change the state, so repeated saves of the same text or value add nothing to the log
func (p *pageState) changes(recs []record) []record {
	res := make([]record, 0, len(recs))
	for _, r := range recs {
		u, ok := p.users[r.User]
		if !ok && r.K == "" && (r.Text == nil || *r.Text == "") {
			continue // a browser that only opened the page is not a user yet
		}
		if ok {
			if r.Text != nil && r.K == "" && *r.Text == u.Text {
				continue
			}
			if v, has := u.Entries[r.K]; r.K != "" && r.Text == nil && has && string(v) == string(r.V) {
				continue
			}
		}
		res = append(res, r)
	}
	return res
}

// check rejects a batch that would push the page past its user and entry limits
func (p *pageState) check(recs []record) error {
	newUsers := map[string]bool{}
	newKeys := map[string]map[string]bool{}
	for _, r := range recs {
		u, ok := p.users[r.User]
		if !ok {
			newUsers[r.User] = true
		}
		if r.K == "" || (ok && u.Entries[r.K] != nil) {
			continue
		}
		if newKeys[r.User] == nil {
			newKeys[r.User] = map[string]bool{}
		}
		newKeys[r.User][r.K] = true
	}
	if len(p.users)+len(newUsers) > maxUsersPerSet {
		return errTooManyUsers
	}
	for id, keys := range newKeys {
		have := 0
		if u, ok := p.users[id]; ok {
			have = len(u.Entries)
		}
		if have+len(keys) > maxEntriesUser {
			return errTooManyEntries
		}
	}
	return nil
}

func marshalRecords(recs []record) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			return nil, fmt.Errorf("marshal record: %w", err)
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	return buf, nil
}

// write appends the records that change something to the page log and applies them;
// a log about to pass the size limit is first compacted to its folded state
func (s *store) write(page string, recs []record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.page(page)
	if err != nil {
		return err
	}
	recs = p.changes(recs)
	if len(recs) == 0 {
		return nil
	}
	if err := p.check(recs); err != nil {
		return err
	}
	buf, err := marshalRecords(recs)
	if err != nil {
		return err
	}
	if p.size+int64(len(buf)) > s.limit {
		if err := s.compact(page, p); err != nil {
			return err
		}
		if p.size+int64(len(buf)) > s.limit {
			return errSetFull
		}
	}
	if _, err := p.file.Write(buf); err != nil {
		return fmt.Errorf("append log: %w", err)
	}
	p.size += int64(len(buf))
	for _, r := range recs {
		p.apply(r)
	}
	return nil
}

// compact replaces the page log with one record per current entry and text, dropping the history of changes;
// caller holds s.mu
func (s *store) compact(page string, p *pageState) error {
	var recs []record
	ids := make([]string, 0, len(p.users))
	for id := range p.users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		u := p.users[id]
		keys := make([]string, 0, len(u.Entries))
		for k := range u.Entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			recs = append(recs, record{TS: u.Updated, User: id, K: k, V: u.Entries[k]})
		}
		if u.Text != "" {
			text := u.Text
			recs = append(recs, record{TS: u.Updated, User: id, Text: &text})
		}
		if len(keys) == 0 && u.Text == "" {
			recs = append(recs, record{TS: u.Updated, User: id})
		}
	}
	buf, err := marshalRecords(recs)
	if err != nil {
		return err
	}
	tmp := s.path(page) + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return fmt.Errorf("write compacted log: %w", err)
	}
	if err := os.Rename(tmp, s.path(page)); err != nil {
		return fmt.Errorf("replace log: %w", err)
	}
	f, err := os.OpenFile(s.path(page), os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("reopen log: %w", err)
	}
	_ = p.file.Close()
	p.file, p.size = f, int64(len(buf))
	log.Printf("[INFO] compacted %s to %d bytes", s.path(page), p.size)
	return nil
}

// get returns a copy of one user's state, nil when the user has none
func (s *store) get(page, id string) (*user, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.page(page)
	if err != nil {
		return nil, err
	}
	u, ok := p.users[id]
	if !ok {
		return nil, nil
	}
	return u.clone(), nil
}

// all returns copies of every user's state, most recently updated first
func (s *store) all(page string) ([]*user, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.page(page)
	if err != nil {
		return nil, err
	}
	res := make([]*user, 0, len(p.users))
	for _, u := range p.users {
		res = append(res, u.clone())
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Updated.After(res[j].Updated) })
	return res, nil
}

// raw returns the page log as written
func (s *store) raw(page string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.page(page); err != nil {
		return nil, err
	}
	return os.ReadFile(s.path(page))
}

func (s *store) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.pages {
		_ = p.file.Close()
	}
}

func (u *user) clone() *user {
	c := *u
	c.Entries = make(map[string]json.RawMessage, len(u.Entries))
	maps.Copy(c.Entries, u.Entries)
	return &c
}
