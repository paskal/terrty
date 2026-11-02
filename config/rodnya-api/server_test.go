package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPage = "0123456789abcdef"
	testKey  = "00112233445566778899aabbccddeeff"
	devA     = "aaaaaaaaaaaaaaaa"
	devB     = "bbbbbbbbbbbbbbbb"
)

func newTestServer(t *testing.T) (*server, string) {
	t.Helper()
	pages, data := t.TempDir(), t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(pages, testPage), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pages, testPage, "index.html"), []byte("x"), 0o600))
	st := newStore(data)
	t.Cleanup(st.close)
	return &server{store: st, pages: pages, adminKey: testKey, limiter: newLimiter(1000, 1000)}, data
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func state(t *testing.T, h http.Handler, dev string) stateResp {
	t.Helper()
	rec := do(t, h, http.MethodGet, "/rodnya/"+testPage+"/api/state?dev="+url.QueryEscape(dev), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var s stateResp
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	return s
}

func TestSaveValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.routes()
	save := "/rodnya/" + testPage + "/api/save"
	tests := []struct {
		name string
		path string
		body string
		code int
	}{
		{"valid", save, `{"dev":"` + devA + `","items":[{"k":"notes|n012","v":"текст"}]}`, http.StatusOK},
		{"valid object value", save, `{"dev":"` + devA + `","items":[{"k":"people|p3","v":{"no":["1"],"ok":true}}]}`, http.StatusOK},
		{"page not published", "/rodnya/fedcba9876543210/api/save", `{"dev":"` + devA + `"}`, http.StatusNotFound},
		{"page too short", "/rodnya/0123/api/save", `{"dev":"` + devA + `"}`, http.StatusNotFound},
		{"bad device", save, `{"dev":"xyz","items":[]}`, http.StatusBadRequest},
		{"unknown field", save, `{"dev":"` + devA + `","admin":true}`, http.StatusBadRequest},
		{"bad key", save, `{"dev":"` + devA + `","items":[{"k":"<script>","v":"1"}]}`, http.StatusBadRequest},
		{"missing value", save, `{"dev":"` + devA + `","items":[{"k":"notes|n1"}]}`, http.StatusBadRequest},
		{"value too long", save, `{"dev":"` + devA + `","items":[{"k":"notes|n1","v":"` + strings.Repeat("я", 8300) + `"}]}`, http.StatusBadRequest},
		{"name is not accepted", save, `{"dev":"` + devA + `","name":"Иван"}`, http.StatusBadRequest},
		{"text too long", save, `{"dev":"` + devA + `","text":"` + strings.Repeat("x", maxTextBytes+1) + `"}`, http.StatusBadRequest},
		{"trailing data", save, `{"dev":"` + devA + `"}{"dev":"` + devA + `"}`, http.StatusBadRequest},
		{"body too large", save, `{"dev":"` + devA + `","text":"` + strings.Repeat("x", maxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge},
		{"too many items", save, `{"dev":"` + devA + `","items":[` + strings.TrimSuffix(strings.Repeat(`{"k":"a|b","v":1},`, maxItems+1), ",") + `]}`,
			http.StatusBadRequest},
		{"get not allowed", save, "", http.StatusMethodNotAllowed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			method := http.MethodPost
			if tc.code == http.StatusMethodNotAllowed {
				method = http.MethodGet
			}
			rec := do(t, h, method, tc.path, tc.body)
			assert.Equal(t, tc.code, rec.Code, rec.Body.String())
		})
	}
}

func TestStateAndRestart(t *testing.T) {
	srv, data := newTestServer(t)
	h := srv.routes()
	save := "/rodnya/" + testPage + "/api/save"

	rec := do(t, h, http.MethodPost, save, `{"dev":"`+devA+`","text":"№ 1: первая","items":[{"k":"notes|n001","v":"первая"},{"k":"faces|n001:2","v":"no"}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	s := state(t, h, devA)
	assert.JSONEq(t, `"первая"`, string(s.Entries["notes|n001"]))
	assert.Empty(t, state(t, h, devB).Entries, "each browser sees only its own answers")
	assert.Equal(t, http.StatusBadRequest, do(t, h, http.MethodGet, "/rodnya/"+testPage+"/api/state?dev=../x", "").Code)

	rec = do(t, h, http.MethodPost, save, `{"dev":"`+devA+`","items":[{"k":"faces|n001:2","v":""}]}`)
	require.Equal(t, http.StatusOK, rec.Code)

	// a new process reads the same state back from the log
	st2 := newStore(data)
	defer st2.close()
	srv2 := &server{store: st2, pages: srv.pages, adminKey: testKey, limiter: newLimiter(1000, 1000)}
	s = state(t, srv2.routes(), devA)
	assert.JSONEq(t, `""`, string(s.Entries["faces|n001:2"]), "a cleared answer stays cleared")
	assert.JSONEq(t, `"первая"`, string(s.Entries["notes|n001"]))
	users, err := st2.all(testPage)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, devA, users[0].ID)
	assert.Equal(t, "№ 1: первая", users[0].Text)
}

func TestOpeningPageCreatesNoUser(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.routes()
	require.Equal(t, http.StatusOK, do(t, h, http.MethodPost, "/rodnya/"+testPage+"/api/save", `{"dev":"`+devA+`","items":[],"text":""}`).Code)
	users, err := srv.store.all(testPage)
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestUnchangedSavesAddNothing(t *testing.T) {
	srv, data := newTestServer(t)
	h := srv.routes()
	body := `{"dev":"` + devA + `","text":"` + strings.Repeat("т", 1000) + `","items":[{"k":"notes|n001","v":"одно"}]}`
	require.Equal(t, http.StatusOK, do(t, h, http.MethodPost, "/rodnya/"+testPage+"/api/save", body).Code)
	fi, err := os.Stat(filepath.Join(data, testPage+".jsonl"))
	require.NoError(t, err)
	for range 5 {
		require.Equal(t, http.StatusOK, do(t, h, http.MethodPost, "/rodnya/"+testPage+"/api/save", body).Code)
	}
	fi2, err := os.Stat(filepath.Join(data, testPage+".jsonl"))
	require.NoError(t, err)
	assert.Equal(t, fi.Size(), fi2.Size(), "repeating the same text and value must not grow the log")
}

func TestCompaction(t *testing.T) {
	dir := t.TempDir()
	st := newStore(dir)
	st.limit = 4096
	for i := range 200 {
		text := "текст " + itoa(i)
		require.NoError(t, st.write(testPage, []record{{User: devA, K: "notes|n001", V: json.RawMessage(`"` + itoa(i) + `"`)}, {User: devA, Text: &text}}))
	}
	require.NoError(t, st.write(testPage, []record{{User: devB, K: "notes|n002", V: json.RawMessage(`"b"`)}}))
	fi, err := os.Stat(filepath.Join(dir, testPage+".jsonl"))
	require.NoError(t, err)
	assert.LessOrEqual(t, fi.Size(), st.limit, "log is compacted instead of growing past the limit")
	st.close()

	st2 := newStore(dir)
	st2.limit = 4096
	defer st2.close()
	u, err := st2.get(testPage, devA)
	require.NoError(t, err)
	assert.JSONEq(t, `"199"`, string(u.Entries["notes|n001"]))
	assert.Equal(t, "текст 199", u.Text)
	u, err = st2.get(testPage, devB)
	require.NoError(t, err)
	assert.JSONEq(t, `"b"`, string(u.Entries["notes|n002"]))

	big := strings.Repeat("x", 5000)
	require.ErrorIs(t, st2.write(testPage, []record{{User: devA, Text: &big}}), errSetFull, "a write that cannot fit even after compaction is refused")
}

func TestAdmin(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.routes()
	rec := do(t, h, http.MethodPost, "/rodnya/"+testPage+"/api/save",
		`{"dev":"`+devA+`","text":"№ 5: это не <b>Пётр</b>","items":[{"k":"notes|n005","v":"это не Пётр"}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	base := "/rodnya/" + testPage + "/api/admin/"

	tests := []struct {
		name, path, want string
		code             int
	}{
		{"summary", base + testKey + "/", devA, http.StatusOK},
		{"summary escapes text", base + testKey + "/", "№ 5: это не &lt;b&gt;Пётр&lt;/b&gt;", http.StatusOK},
		{"export", base + testKey + "/export.json", `"notes|n005":"это не Пётр"`, http.StatusOK},
		{"log", base + testKey + "/log.jsonl", `"k":"notes|n005"`, http.StatusOK},
		{"wrong key", base + strings.Repeat("0", 32) + "/", "", http.StatusNotFound},
		{"short key", base + "00/", "", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, tc.path, "")
			assert.Equal(t, tc.code, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}

	srv.adminKey = ""
	assert.Equal(t, http.StatusNotFound, do(t, srv.routes(), http.MethodGet, base+testKey+"/", "").Code, "empty key disables the summary")
}

func TestRateLimit(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.limiter = newLimiter(0.001, 3)
	h := srv.routes()
	codes := []int{}
	for range 5 {
		req := httptest.NewRequest(http.MethodGet, "/rodnya/"+testPage+"/api/state?dev="+devA, http.NoBody)
		req.Header.Set("X-Real-IP", "192.0.2.1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	assert.Equal(t, []int{200, 200, 200, 429, 429}, codes)

	req := httptest.NewRequest(http.MethodGet, "/rodnya/"+testPage+"/api/state?dev="+devA, http.NoBody)
	req.Header.Set("X-Real-IP", "192.0.2.2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "another client has its own bucket")
}

func TestStoreLimits(t *testing.T) {
	st := newStore(t.TempDir())
	defer st.close()
	recs := make([]record, 0, maxEntriesUser+1)
	for i := range maxEntriesUser + 1 {
		recs = append(recs, record{User: "n:x", K: "a|k" + itoa(i), V: json.RawMessage(`1`)})
	}
	require.ErrorIs(t, st.write(testPage, recs), errTooManyEntries)
	require.NoError(t, st.write(testPage, recs[:maxEntriesUser]))
	require.ErrorIs(t, st.write(testPage, recs[maxEntriesUser:]), errTooManyEntries)
	require.NoError(t, st.write(testPage, recs[:1]), "rewriting an existing key is not a new entry")

	users := make([]record, 0, maxUsersPerSet)
	for i := range maxUsersPerSet {
		users = append(users, record{User: "u" + itoa(i), K: "a|k", V: json.RawMessage(`1`)})
	}
	require.ErrorIs(t, st.write(testPage, users), errTooManyUsers)
}

func TestBadLogLineSkipped(t *testing.T) {
	dir := t.TempDir()
	good := `{"ts":"2026-11-02T10:00:00Z","user":"n:a","k":"notes|n1","v":"x"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, testPage+".jsonl"), []byte("not json\n"+good+"\n"), 0o600))
	st := newStore(dir)
	defer st.close()
	u, err := st.get(testPage, "n:a")
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.JSONEq(t, `"x"`, string(u.Entries["notes|n1"]))
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
