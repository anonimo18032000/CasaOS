package v1

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/labstack/echo/v4"
)

// Walks over big or remote trees can take very long; both endpoints stop at a deadline and say so
// in the response instead of hanging the request.
const (
	usageDeadline  = 20 * time.Second
	searchDeadline = 15 * time.Second
	usageMaxItems  = 200
)

// pseudo filesystems that never hold user data and would make a walk of "/" endless
var skippedPseudoDirs = map[string]bool{"/proc": true, "/sys": true, "/dev": true, "/run": true}

type UsageItem struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

type UsageResult struct {
	Path       string      `json:"path"`
	Total      int64       `json:"total"`
	Items      []UsageItem `json:"items"`
	Partial    bool        `json:"partial"`               // the deadline hit before everything was counted
	Hidden     int         `json:"hidden,omitempty"`      // entries folded away because there were more than usageMaxItems
	HiddenSize int64       `json:"hidden_size,omitempty"` // their combined size
}

type requestError string

func (e requestError) Error() string { return string(e) }

// requireAbsDir cleans a user-supplied path and makes sure it is an absolute, existing directory.
func requireAbsDir(raw string) (string, error) {
	if raw == "" {
		return "", requestError("path is empty")
	}
	cleaned := filepath.Clean(raw)
	if !filepath.IsAbs(cleaned) {
		return "", requestError("path must be absolute")
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", requestError(err.Error())
	}
	if !info.IsDir() {
		return "", requestError("path is not a directory")
	}
	return cleaned, nil
}

func errorJSON(ctx echo.Context, err error) error {
	if _, ok := err.(requestError); ok {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: err.Error()})
	}
	return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR), Data: err.Error()})
}

// treeSize adds up the regular files below dir. Symlinks are not followed, unreadable entries are
// skipped, and the walk stops when ctx ends (the caller then reports a partial result).
func treeSize(ctx context.Context, dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() && skippedPseudoDirs[path] {
			return fs.SkipDir
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// GetFolderUsage lists the entries directly inside `path` with their (recursive) sizes - the
// numbers behind the disk usage treemap. Directories are measured concurrently.
func GetFolderUsage(ctx echo.Context) error {
	root, err := requireAbsDir(ctx.QueryParam("path"))
	if err != nil {
		return errorJSON(ctx, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return errorJSON(ctx, err)
	}

	deadline, cancel := context.WithTimeout(ctx.Request().Context(), usageDeadline)
	defer cancel()

	items := make([]UsageItem, len(entries))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // a few walks at a time: this is I/O bound and often on a slow disk
	for i, entry := range entries {
		full := filepath.Join(root, entry.Name())
		items[i] = UsageItem{Name: entry.Name(), Path: full, IsDir: entry.IsDir()}
		if entry.IsDir() {
			if skippedPseudoDirs[full] {
				continue
			}
			wg.Add(1)
			go func(i int, full string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-deadline.Done():
					return
				}
				items[i].Size = treeSize(deadline, full)
			}(i, full)
		} else if entry.Type().IsRegular() {
			if info, err := entry.Info(); err == nil {
				items[i].Size = info.Size()
			}
		}
	}
	wg.Wait()

	result := UsageResult{Path: root, Partial: deadline.Err() != nil}
	kept := items[:0]
	for _, it := range items {
		if it.Size > 0 {
			kept = append(kept, it)
			result.Total += it.Size
		}
	}
	sort.Slice(kept, func(a, b int) bool { return kept[a].Size > kept[b].Size })
	if len(kept) > usageMaxItems {
		for _, it := range kept[usageMaxItems:] {
			result.Hidden++
			result.HiddenSize += it.Size
		}
		kept = kept[:usageMaxItems]
	}
	result.Items = kept
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: result})
}

// ---- search ----

var searchTypeExtensions = map[string][]string{
	"image":   {".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".heic", ".tiff", ".avif"},
	"video":   {".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts"},
	"audio":   {".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a", ".opus", ".wma"},
	"doc":     {".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".txt", ".md", ".rtf", ".csv"},
	"archive": {".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz", ".iso"},
	"code":    {".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".rs", ".sh", ".json", ".yaml", ".yml", ".html", ".css", ".vue", ".php", ".sql"},
}

type SearchItem struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified int64  `json:"modified"` // unix seconds
}

type SearchResult struct {
	Items   []SearchItem `json:"items"`
	Partial bool         `json:"partial"` // stopped early: deadline or result limit
}

func matchesType(kind, name string, isDir bool) bool {
	switch kind {
	case "", "any":
		return true
	case "folder":
		return isDir
	}
	if isDir {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range searchTypeExtensions[kind] {
		if e == ext {
			return true
		}
	}
	return false
}

func queryInt64(ctx echo.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(ctx.QueryParam(name), 10, 64)
	return v, err == nil
}

// SearchFiles walks `path` looking for names containing `q` (case-insensitive), optionally filtered
// by `type`, `min_size`/`max_size` (bytes) and `from`/`to` (modification time, unix seconds).
func SearchFiles(ctx echo.Context) error {
	root, err := requireAbsDir(ctx.QueryParam("path"))
	if err != nil {
		return errorJSON(ctx, err)
	}
	query := strings.ToLower(strings.TrimSpace(ctx.QueryParam("q")))
	kind := ctx.QueryParam("type")
	minSize, hasMin := queryInt64(ctx, "min_size")
	maxSize, hasMax := queryInt64(ctx, "max_size")
	from, hasFrom := queryInt64(ctx, "from")
	to, hasTo := queryInt64(ctx, "to")
	if query == "" && (kind == "" || kind == "any") && !hasMin && !hasMax && !hasFrom && !hasTo {
		return errorJSON(ctx, requestError("give a name, a type or a filter"))
	}
	limit := 200
	if n, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && n > 0 && n <= 1000 {
		limit = n
	}

	deadline, cancel := context.WithTimeout(ctx.Request().Context(), searchDeadline)
	defer cancel()

	result := SearchResult{Items: []SearchItem{}}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if deadline.Err() != nil {
			result.Partial = true
			return deadline.Err()
		}
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		if d.IsDir() && skippedPseudoDirs[path] {
			return fs.SkipDir
		}
		name := d.Name()
		if query != "" && !strings.Contains(strings.ToLower(name), query) {
			return nil
		}
		isDir := d.IsDir()
		if !matchesType(kind, name, isDir) {
			return nil
		}
		if isDir && (hasMin || hasMax) {
			return nil // a folder's size would need a walk of its own
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if (hasMin && info.Size() < minSize) || (hasMax && info.Size() > maxSize) {
			return nil
		}
		mod := info.ModTime().Unix()
		if (hasFrom && mod < from) || (hasTo && mod > to) {
			return nil
		}
		result.Items = append(result.Items, SearchItem{Name: name, Path: path, Size: info.Size(), IsDir: isDir, Modified: mod})
		if len(result.Items) >= limit {
			result.Partial = true
			return fs.SkipAll
		}
		return nil
	})
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: result})
}
