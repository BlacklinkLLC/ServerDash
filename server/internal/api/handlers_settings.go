package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName    = "ServerDash"
	maxAppNameLength  = 60
	maxLogoUploadSize = 2 << 20 // 2MB — plenty for a logo, small enough to not be a DoS vector
)

// logoContentTypes maps accepted upload content types to the file
// extension they're stored under.
var logoContentTypes = map[string]string{
	"image/png":     "png",
	"image/jpeg":    "jpg",
	"image/svg+xml": "svg",
	"image/webp":    "webp",
	"image/x-icon":  "ico",
}

func (s *Server) brandingDir() string {
	return filepath.Join(s.cfg.DataDir, "branding")
}

// settingsResponse is what both the public GET /api/settings and the admin
// PUT return: enough for the login/setup screens (unauthenticated) to
// render the operator's branding before anyone has signed in.
type settingsResponse struct {
	AppName string  `json:"appName"`
	LogoURL *string `json:"logoUrl,omitempty"`
}

func (s *Server) currentSettings(r *http.Request) settingsResponse {
	appName := defaultAppName
	if v, ok, _ := s.store.GetSetting(r.Context(), "app_name"); ok && v != "" {
		appName = v
	}

	resp := settingsResponse{AppName: appName}
	if ext, ok, _ := s.store.GetSetting(r.Context(), "logo_ext"); ok && ext != "" {
		version, _, _ := s.store.GetSetting(r.Context(), "logo_updated_at")
		url := "/api/branding/logo?v=" + version
		resp.LogoURL = &url
	}
	return resp
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.currentSettings(r))
}

type updateSettingsRequest struct {
	AppName string `json:"appName"`
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(req.AppName)
	if name == "" {
		name = defaultAppName
	}
	if len(name) > maxAppNameLength {
		writeError(w, http.StatusBadRequest, fmt.Errorf("app name must be %d characters or fewer", maxAppNameLength))
		return
	}

	if err := s.store.SetSetting(r.Context(), "app_name", name); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.currentSettings(r))
}

func (s *Server) handleUploadLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoUploadSize)
	if err := r.ParseMultipartForm(maxLogoUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("logo file too large or malformed upload"))
		return
	}
	file, header, err := r.FormFile("logo")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("missing \"logo\" file in upload"))
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	ext, ok := logoContentTypes[contentType]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported image type %q (use PNG, JPEG, SVG, WebP, or ICO)", contentType))
		return
	}

	if err := os.MkdirAll(s.brandingDir(), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	removeExistingLogo(s.brandingDir())

	dest, err := os.Create(filepath.Join(s.brandingDir(), "logo."+ext))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer dest.Close()
	if _, err := io.Copy(dest, file); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.store.SetSetting(r.Context(), "logo_ext", ext); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SetSetting(r.Context(), "logo_updated_at", strconv.FormatInt(time.Now().UnixNano(), 36)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, s.currentSettings(r))
}

func (s *Server) handleDeleteLogo(w http.ResponseWriter, r *http.Request) {
	removeExistingLogo(s.brandingDir())
	_ = s.store.DeleteSetting(r.Context(), "logo_ext")
	_ = s.store.DeleteSetting(r.Context(), "logo_updated_at")
	writeJSON(w, http.StatusOK, s.currentSettings(r))
}

func removeExistingLogo(dir string) {
	for _, ext := range logoContentTypes {
		_ = os.Remove(filepath.Join(dir, "logo."+ext))
	}
}

// handleServeLogo is public (unauthenticated) so the setup and login
// screens, which render before anyone can sign in, can show the operator's
// branding.
func (s *Server) handleServeLogo(w http.ResponseWriter, r *http.Request) {
	ext, ok, err := s.store.GetSetting(r.Context(), "logo_ext")
	if err != nil || !ok || ext == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(s.brandingDir(), "logo."+ext))
}
