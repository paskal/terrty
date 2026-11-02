package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	log "github.com/go-pkgz/lgr"
	"golang.org/x/time/rate"
)

const (
	maxBodyBytes  = 256 << 10
	maxItems      = 1000
	maxValueBytes = 16 << 10
	maxTextBytes  = 200 << 10
)

var (
	pageRe     = regexp.MustCompile(`^[0-9a-f]{16}$`)
	adminKeyRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
	deviceRe   = regexp.MustCompile(`^[0-9a-f]{16,32}$`)
	itemKeyRe  = regexp.MustCompile(`^[a-z]{1,16}\|[A-Za-z0-9:_#.\-]{1,64}$`)
)

type server struct {
	store    *store
	pages    string
	adminKey string
	limiter  *limiter
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("pong")) })
	api := http.NewServeMux()
	api.HandleFunc("GET /rodnya/{page}/api/state", s.page(s.handleState))
	api.HandleFunc("POST /rodnya/{page}/api/save", s.page(s.handleSave))
	api.HandleFunc("GET /rodnya/{page}/api/admin/{key}/{$}", s.page(s.admin(s.handleSummary)))
	api.HandleFunc("GET /rodnya/{page}/api/admin/{key}/export.json", s.page(s.admin(s.handleExport)))
	api.HandleFunc("GET /rodnya/{page}/api/admin/{key}/log.jsonl", s.page(s.admin(s.handleLog)))
	mux.Handle("/rodnya/", s.limiter.middleware(api))
	return mux
}

// page lets a request through only for a page that is published, so a guessed address stores nothing
func (s *server) page(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		p := r.PathValue("page")
		if !pageRe.MatchString(p) {
			http.NotFound(w, r)
			return
		}
		if fi, err := os.Stat(filepath.Join(s.pages, p, "index.html")); err != nil || !fi.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func (s *server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if s.adminKey == "" || subtle.ConstantTimeCompare([]byte(key), []byte(s.adminKey)) != 1 {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

type stateResp struct {
	Updated *time.Time                 `json:"updated,omitempty"`
	Entries map[string]json.RawMessage `json:"entries"`
}

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	dev := r.URL.Query().Get("dev")
	if !deviceRe.MatchString(dev) {
		jsonError(w, http.StatusBadRequest, "bad device id")
		return
	}
	u, err := s.store.get(r.PathValue("page"), dev)
	if err != nil {
		log.Printf("[ERROR] state: %v", err)
		jsonError(w, http.StatusInternalServerError, "storage error")
		return
	}
	resp := stateResp{Entries: map[string]json.RawMessage{}}
	if u != nil {
		resp.Updated, resp.Entries = &u.Updated, u.Entries
	}
	writeJSON(w, http.StatusOK, resp)
}

type saveItem struct {
	K string          `json:"k"`
	V json.RawMessage `json:"v"`
}

type saveReq struct {
	Dev   string     `json:"dev"`  // random id the browser keeps in localStorage; it is the only identity
	Text  *string    `json:"text"` // the page's own plain-text summary of this browser's answers
	Items []saveItem `json:"items"`
}

func (q *saveReq) validate() error {
	if !deviceRe.MatchString(q.Dev) {
		return errors.New("bad device id")
	}
	if len(q.Items) > maxItems {
		return errors.New("too many items")
	}
	for _, it := range q.Items {
		if !itemKeyRe.MatchString(it.K) {
			return errors.New("bad item key")
		}
		if len(it.V) == 0 || len(it.V) > maxValueBytes || !json.Valid(it.V) {
			return errors.New("bad item value")
		}
	}
	if q.Text != nil && (len(*q.Text) > maxTextBytes || !utf8.ValidString(*q.Text)) {
		return errors.New("bad text")
	}
	return nil
}

func (s *server) handleSave(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var q saveReq
	if err := dec.Decode(&q); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			jsonError(w, http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		jsonError(w, http.StatusBadRequest, "bad json")
		return
	}
	if dec.More() {
		jsonError(w, http.StatusBadRequest, "bad json")
		return
	}
	if err := q.validate(); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now().UTC()
	recs := make([]record, 0, len(q.Items)+1)
	for _, it := range q.Items {
		recs = append(recs, record{TS: now, User: q.Dev, K: it.K, V: compactJSON(it.V)})
	}
	if q.Text != nil {
		recs = append(recs, record{TS: now, User: q.Dev, Text: q.Text})
	}
	if err := s.store.write(r.PathValue("page"), recs); err != nil {
		if errors.Is(err, errTooManyUsers) || errors.Is(err, errTooManyEntries) || errors.Is(err, errSetFull) {
			jsonError(w, http.StatusInsufficientStorage, err.Error())
			return
		}
		log.Printf("[ERROR] save: %v", err)
		jsonError(w, http.StatusInternalServerError, "storage error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": now})
}

var summaryTmpl = template.Must(template.New("summary").Funcs(template.FuncMap{
	"keys": func(m map[string]json.RawMessage) []string {
		ks := make([]string, 0, len(m))
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return ks
	},
	"val":  func(m map[string]json.RawMessage, k string) string { return string(m[k]) },
	"when": func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") },
}).Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1"><meta name="robots" content="noindex,nofollow,noarchive">
<title>Answers: {{len .Users}}</title>
<style>
:root{color-scheme:light dark;--line:#8884}body{font:16px/1.45 system-ui,sans-serif;max-width:1100px;margin:0 auto;padding:16px}
section{border:1px solid var(--line);border-radius:8px;padding:10px 14px;margin:12px 0}h2{margin:0 0 4px;font-size:19px}
.m{opacity:.7;font-size:14px}pre{white-space:pre-wrap;background:#8881;padding:8px;border-radius:6px}
table{border-collapse:collapse;font-size:13px}td{border-top:1px solid var(--line);padding:3px 8px;vertical-align:top;word-break:break-word}
</style></head><body>
<h1>Browsers with answers on this page: {{len .Users}}</h1>
<p><a href="export.json" download>export.json</a> (state per person) · <a href="log.jsonl" download>log.jsonl</a> (every change)</p>
{{range .Users}}<section>
<h2>{{.ID}}</h2>
<p class="m">updated {{when .Updated}} · entries: {{len .Entries}}</p>
{{if .Text}}<pre>{{.Text}}</pre>{{else}}<p class="m">no summary text</p>{{end}}
<details><summary>entries</summary><table>{{$e := .Entries}}{{range keys $e}}<tr><td>{{.}}</td><td>{{val $e .}}</td></tr>{{end}}</table></details>
</section>{{else}}<p>No answers yet.</p>{{end}}
</body></html>`))

func (s *server) handleSummary(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.all(r.PathValue("page"))
	if err != nil {
		log.Printf("[ERROR] summary: %v", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := summaryTmpl.Execute(w, map[string]any{"Users": users}); err != nil {
		log.Printf("[WARN] summary template: %v", err)
	}
}

func (s *server) handleExport(w http.ResponseWriter, r *http.Request) {
	page := r.PathValue("page")
	users, err := s.store.all(page)
	if err != nil {
		log.Printf("[ERROR] export: %v", err)
		jsonError(w, http.StatusInternalServerError, "storage error")
		return
	}
	now := time.Now().UTC()
	w.Header().Set("Content-Disposition", `attachment; filename="answers-`+page+"-"+now.Format("20060102-1504")+`.json"`)
	writeJSON(w, http.StatusOK, map[string]any{"page": page, "exported": now, "users": users})
}

func (s *server) handleLog(w http.ResponseWriter, r *http.Request) {
	page := r.PathValue("page")
	b, err := s.store.raw(page)
	if err != nil {
		log.Printf("[ERROR] log: %v", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="answers-`+page+"-"+time.Now().UTC().Format("20060102-1504")+`.jsonl"`)
	_, _ = w.Write(b)
}

func compactJSON(v json.RawMessage) json.RawMessage {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	var x any
	if err := json.Unmarshal(v, &x); err != nil || enc.Encode(x) != nil {
		return v
	}
	return json.RawMessage(strings.TrimSuffix(b.String(), "\n"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// limiter is a per-client token bucket; the client address comes from X-Real-IP set by nginx
type limiter struct {
	mu      sync.Mutex
	rps     rate.Limit
	burst   int
	clients map[string]*client
}

type client struct {
	lim  *rate.Limiter
	seen time.Time
}

func newLimiter(rps float64, burst int) *limiter {
	return &limiter{rps: rate.Limit(rps), burst: burst, clients: map[string]*client{}}
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.clients[ip]
	if !ok {
		c = &client{lim: rate.NewLimiter(l.rps, l.burst)}
		l.clients[ip] = c
	}
	c.seen = time.Now()
	return c.lim.Allow()
}

func (l *limiter) cleanupLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.mu.Lock()
			for ip, c := range l.clients {
				if time.Since(c.seen) > every {
					delete(l.clients, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

func (l *limiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		}
		if !l.allow(ip) {
			jsonError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}
