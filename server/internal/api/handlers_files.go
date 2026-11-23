package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type fileEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"isDir"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified string `json:"modified"`
	Mime     string `json:"mime"`
}

type fileListing struct {
	Path    string      `json:"path"`
	Parent  string      `json:"parent"`
	Entries []fileEntry `json:"entries"`
}

func (s *Server) handleFileList(w http.ResponseWriter, r *http.Request) {
	display, real, err := s.resolveHostPath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		childPath := path.Join(display, entry.Name())
		mimeType := "directory"
		if !info.IsDir() {
			mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(entry.Name())))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
		}
		out = append(out, fileEntry{
			Name:     entry.Name(),
			Path:     childPath,
			IsDir:    info.IsDir(),
			Size:     info.Size(),
			Mode:     info.Mode().String(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
			Mime:     mimeType,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	parent := path.Dir(display)
	if parent == "." {
		parent = "/"
	}
	writeJSON(w, http.StatusOK, fileListing{Path: display, Parent: parent, Entries: out})
}

func (s *Server) handleFileContent(w http.ResponseWriter, r *http.Request) {
	_, real, err := s.resolveHostPath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := os.Stat(real)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, errors.New("path is a directory"))
		return
	}

	if s.cfg.HostRoot != "" {
		real, err = filepath.EvalSymlinks(real)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !s.withinHostRoot(real) {
			writeError(w, http.StatusForbidden, errors.New("symlink escapes host root"))
			return
		}
	}

	file, err := os.Open(real)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	if contentType == "" {
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		_, _ = file.Seek(0, io.SeekStart)
		contentType = http.DetectContentType(buf[:n])
	}
	if strings.HasPrefix(contentType, "text/") && !strings.Contains(contentType, "charset=") {
		contentType += "; charset=utf-8"
	}

	disposition := "inline"
	if r.URL.Query().Get("download") == "1" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, url.PathEscape(info.Name())))
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (s *Server) resolveHostPath(requested string) (display string, real string, err error) {
	if requested == "" {
		requested = "/"
	}
	if !filepath.IsAbs(requested) {
		requested = "/" + requested
	}
	display = filepath.ToSlash(filepath.Clean(filepath.FromSlash(requested)))
	real = filepath.FromSlash(display)
	if root := s.cfg.HostRoot; root != "" {
		real = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(display, "/")))
		if !s.withinHostRoot(real) {
			return "", "", errors.New("path escapes host root")
		}
	}
	return display, real, nil
}

func (s *Server) withinHostRoot(candidate string) bool {
	root, err := filepath.Abs(s.cfg.HostRoot)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
