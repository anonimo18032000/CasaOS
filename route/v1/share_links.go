package v1

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Temporary public download links for single files. The management endpoints (list/create/delete)
// sit behind the normal login; only the download endpoint (/v1/pubshare/:token) is public, and it
// only ever serves the one file a link was created for.

const (
	minShareSeconds = 60
	maxShareSeconds = 30 * 24 * 3600
	minSharePassLen = 4
	maxFailedTries  = 5
	lockoutDuration = 5 * time.Minute
)

type shareLink struct {
	Token        string `json:"token"`
	Path         string `json:"path"`
	Name         string `json:"name"`
	PasswordHash string `json:"password_hash,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    int64  `json:"expires_at"`
	MaxDownloads int    `json:"max_downloads"` // 0 = unlimited
	Downloads    int    `json:"downloads"`
}

// shareLinkView is what the API returns: everything except the password hash.
type shareLinkView struct {
	Token        string `json:"token"`
	Path         string `json:"path"`
	Name         string `json:"name"`
	HasPassword  bool   `json:"has_password"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    int64  `json:"expires_at"`
	MaxDownloads int    `json:"max_downloads"`
	Downloads    int    `json:"downloads"`
}

type shareStore struct {
	mu       sync.Mutex
	loaded   bool
	links    map[string]*shareLink
	failures map[string]*failureState // per token
}

type failureState struct {
	count       int
	lockedUntil time.Time
}

var shares = &shareStore{links: map[string]*shareLink{}, failures: map[string]*failureState{}}

func shareLinksPath() string {
	return filepath.Join(constants.DefaultDataPath, "share_links.json")
}

// ensureLoaded must be called with s.mu held.
func (s *shareStore) ensureLoaded() {
	if s.loaded {
		return
	}
	s.loaded = true
	data, err := os.ReadFile(shareLinksPath())
	if err != nil {
		return
	}
	var list []*shareLink
	if err := json.Unmarshal(data, &list); err != nil {
		logger.Error("failed to parse share links file", zap.Error(err))
		return
	}
	for _, l := range list {
		s.links[l.Token] = l
	}
}

// save must be called with s.mu held. Written to a temp file and renamed, like the backup jobs.
func (s *shareStore) save() error {
	list := make([]*shareLink, 0, len(s.links))
	for _, l := range s.links {
		list = append(list, l)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt < list[j].CreatedAt })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	path := shareLinksPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (l *shareLink) usable(now time.Time) bool {
	if now.Unix() >= l.ExpiresAt {
		return false
	}
	return l.MaxDownloads == 0 || l.Downloads < l.MaxDownloads
}

// pruneLocked removes links that can no longer be used. Caller holds s.mu.
func (s *shareStore) pruneLocked(now time.Time) bool {
	changed := false
	for token, l := range s.links {
		if !l.usable(now) {
			delete(s.links, token)
			delete(s.failures, token)
			changed = true
		}
	}
	return changed
}

func newShareToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func toView(l *shareLink) shareLinkView {
	return shareLinkView{
		Token: l.Token, Path: l.Path, Name: l.Name, HasPassword: l.PasswordHash != "",
		CreatedAt: l.CreatedAt, ExpiresAt: l.ExpiresAt, MaxDownloads: l.MaxDownloads, Downloads: l.Downloads,
	}
}

func shareOK(ctx echo.Context, data interface{}) error {
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: data})
}

func shareClientError(ctx echo.Context, msg string) error {
	return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: msg})
}

// checkShareFile verifies that path is a regular file inside /DATA or /mnt, after resolving
// symlinks. It is used both when a link is created and again on every download, because the file
// can be replaced by a symlink to something else in between.
func checkShareFile(path string) (string, error) {
	cleaned, err := validateBackupSourcePath(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", requestError("only files can be shared, not folders")
	}
	return resolved, nil
}

// ListShareLinks returns the links that are still usable.
func ListShareLinks(ctx echo.Context) error {
	shares.mu.Lock()
	defer shares.mu.Unlock()
	shares.ensureLoaded()
	if shares.pruneLocked(time.Now()) {
		_ = shares.save()
	}
	views := make([]shareLinkView, 0, len(shares.links))
	for _, l := range shares.links {
		views = append(views, toView(l))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].CreatedAt > views[j].CreatedAt })
	return shareOK(ctx, views)
}

// CreateShareLink creates a link for one file.
func CreateShareLink(ctx echo.Context) error {
	var req struct {
		Path         string `json:"path"`
		ExpiresIn    int64  `json:"expires_in"` // seconds
		Password     string `json:"password"`
		MaxDownloads int    `json:"max_downloads"`
	}
	if err := ctx.Bind(&req); err != nil {
		return shareClientError(ctx, "invalid request")
	}
	if req.ExpiresIn < minShareSeconds || req.ExpiresIn > maxShareSeconds {
		return shareClientError(ctx, "expires_in must be between 1 minute and 30 days")
	}
	if req.MaxDownloads < 0 {
		return shareClientError(ctx, "max_downloads cannot be negative")
	}
	if req.Password != "" && len(req.Password) < minSharePassLen {
		return shareClientError(ctx, "password is too short")
	}
	resolved, err := checkShareFile(req.Path)
	if err != nil {
		if _, ok := err.(*backupValidationError); ok {
			return shareClientError(ctx, "only files inside /DATA or /mnt can be shared")
		}
		return errorJSON(ctx, err)
	}

	link := &shareLink{Path: resolved, Name: filepath.Base(resolved), MaxDownloads: req.MaxDownloads}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return errorJSON(ctx, err)
		}
		link.PasswordHash = string(hash)
	}
	if link.Token, err = newShareToken(); err != nil {
		return errorJSON(ctx, err)
	}
	now := time.Now()
	link.CreatedAt = now.Unix()
	link.ExpiresAt = now.Add(time.Duration(req.ExpiresIn) * time.Second).Unix()

	shares.mu.Lock()
	defer shares.mu.Unlock()
	shares.ensureLoaded()
	shares.pruneLocked(now)
	shares.links[link.Token] = link
	if err := shares.save(); err != nil {
		delete(shares.links, link.Token)
		return errorJSON(ctx, err)
	}
	return shareOK(ctx, toView(link))
}

// DeleteShareLink revokes a link immediately.
func DeleteShareLink(ctx echo.Context) error {
	token := ctx.Param("token")
	shares.mu.Lock()
	defer shares.mu.Unlock()
	shares.ensureLoaded()
	if _, ok := shares.links[token]; !ok {
		return shareClientError(ctx, "link not found")
	}
	delete(shares.links, token)
	delete(shares.failures, token)
	if err := shares.save(); err != nil {
		return errorJSON(ctx, err)
	}
	return shareOK(ctx, "success")
}

// ---- public side ----

var passwordPage = template.Must(template.New("pw").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Shared file</title>
<style>body{font-family:system-ui,sans-serif;background:#f5f6f8;display:flex;min-height:100vh;align-items:center;justify-content:center;margin:0}
form{background:#fff;padding:2rem;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.1);width:min(22rem,90vw)}
h1{font-size:1.1rem;margin:0 0 .25rem}p{color:#666;margin:.25rem 0 1rem;word-break:break-all}
input,button{width:100%;box-sizing:border-box;padding:.7rem;font-size:1rem;border-radius:8px;border:1px solid #ccc}
button{background:#2f6bff;color:#fff;border:0;margin-top:.75rem;cursor:pointer}.err{color:#c0392b;margin-top:.5rem}</style></head>
<body><form method="post"><h1>This file is password protected</h1><p>{{.Name}}</p>
<input type="password" name="password" placeholder="Password" autofocus required>
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<button type="submit">Download</button></form></body></html>`))

func writeSharePage(c echo.Context, status int, name, errMsg string) error {
	h := c.Response().Header()
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	c.Response().Status = status
	h.Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(status)
	return passwordPage.Execute(c.Response(), map[string]string{"Name": name, "Error": errMsg})
}

func shareNotFound(c echo.Context) error {
	return c.String(http.StatusNotFound, "This link does not exist or has expired.")
}

// PublicShareDownload serves a shared file. GET downloads straight away when the link has no
// password, or shows a password form; POST takes the password from the form.
func PublicShareDownload(c echo.Context) error {
	token := c.Param("token")
	now := time.Now()

	shares.mu.Lock()
	shares.ensureLoaded()
	link, ok := shares.links[token]
	if !ok || !link.usable(now) {
		shares.mu.Unlock()
		return shareNotFound(c)
	}
	snapshot := *link
	state := shares.failures[token]
	locked := state != nil && now.Before(state.lockedUntil)
	shares.mu.Unlock()

	if snapshot.PasswordHash != "" {
		if locked {
			return c.String(http.StatusTooManyRequests, "Too many wrong passwords. Try again in a few minutes.")
		}
		if c.Request().Method != http.MethodPost {
			return writeSharePage(c, http.StatusOK, snapshot.Name, "")
		}
		if bcrypt.CompareHashAndPassword([]byte(snapshot.PasswordHash), []byte(c.FormValue("password"))) != nil {
			shares.mu.Lock()
			st := shares.failures[token]
			if st == nil {
				st = &failureState{}
				shares.failures[token] = st
			}
			st.count++
			if st.count >= maxFailedTries {
				st.count = 0
				st.lockedUntil = time.Now().Add(lockoutDuration)
			}
			shares.mu.Unlock()
			return writeSharePage(c, http.StatusUnauthorized, snapshot.Name, "Wrong password.")
		}
	}

	resolved, err := checkShareFile(snapshot.Path)
	if err != nil {
		return shareNotFound(c)
	}
	f, err := os.Open(resolved)
	if err != nil {
		return shareNotFound(c)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return shareNotFound(c)
	}

	// resumed downloads (Range requests that don't start at 0) are the same download, not a new one
	if rng := c.Request().Header.Get("Range"); rng == "" || strings.HasPrefix(rng, "bytes=0-") {
		shares.mu.Lock()
		if l, ok := shares.links[token]; ok {
			l.Downloads++
			delete(shares.failures, token)
			_ = shares.save()
		}
		shares.mu.Unlock()
	}

	h := c.Response().Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": snapshot.Name}))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	http.ServeContent(c.Response(), c.Request(), snapshot.Name, info.ModTime(), f)
	return nil
}
