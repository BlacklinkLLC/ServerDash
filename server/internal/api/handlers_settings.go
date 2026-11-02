package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName    = "ServerDash"
	defaultAccent     = "#ffb020" // NOVA's amber — matches app.css's --accent default
	maxAppNameLength  = 60
	maxLogoUploadSize = 2 << 20 // 2MB — plenty for a logo, small enough to not be a DoS vector
)

var (
	validThemes    = map[string]bool{"dark": true, "midnight": true, "light": true, "terminal": true}
	validDensities = map[string]bool{"comfortable": true, "compact": true}
	// validWidgets are the overview's widget ids; the order an admin saves
	// them in is the order they render.
	validWidgets = []string{"health", "gauges", "cpuChart", "memChart", "netChart", "cores", "disks", "containers", "processes"}
	maxFooterLen = 200
)

var accentColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

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
// render the operator's branding before anyone has signed in, plus the
// appearance/customization options the dashboard reads once signed in.
type settingsResponse struct {
	AppName        string   `json:"appName"`
	LogoURL        *string  `json:"logoUrl,omitempty"`
	AccentColor    string   `json:"accentColor"`
	ShowSystem     bool     `json:"showSystem"`
	ShowContainers bool     `json:"showContainers"`
	Theme          string   `json:"theme"`
	Density        string   `json:"density"`
	RefreshSeconds int      `json:"refreshSeconds"`
	WarnThreshold  int      `json:"warnThreshold"`
	CritThreshold  int      `json:"critThreshold"`
	Widgets        []string `json:"widgets"`
	FooterText     string   `json:"footerText"`
}

func settingString(s *Server, r *http.Request, key, def string) string {
	if v, ok, _ := s.store.GetSetting(r.Context(), key); ok && v != "" {
		return v
	}
	return def
}

func settingInt(s *Server, r *http.Request, key string, def int) int {
	if v, ok, _ := s.store.GetSetting(r.Context(), key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func (s *Server) currentSettings(r *http.Request) settingsResponse {
	appName := defaultAppName
	if v, ok, _ := s.store.GetSetting(r.Context(), "app_name"); ok && v != "" {
		appName = v
	}
	accent := defaultAccent
	if v, ok, _ := s.store.GetSetting(r.Context(), "accent_color"); ok && v != "" {
		accent = v
	}

	resp := settingsResponse{
		AppName:        appName,
		AccentColor:    accent,
		ShowSystem:     settingBool(s, r, "dashboard_show_system", true),
		ShowContainers: settingBool(s, r, "dashboard_show_containers", true),
		Theme:          settingString(s, r, "theme", "dark"),
		Density:        settingString(s, r, "density", "comfortable"),
		RefreshSeconds: settingInt(s, r, "refresh_seconds", 5),
		WarnThreshold:  settingInt(s, r, "warn_threshold", 75),
		CritThreshold:  settingInt(s, r, "crit_threshold", 90),
		Widgets:        validWidgets,
		FooterText:     settingString(s, r, "footer_text", ""),
	}
	if v, ok, _ := s.store.GetSetting(r.Context(), "overview_widgets"); ok {
		var ws []string
		if json.Unmarshal([]byte(v), &ws) == nil {
			resp.Widgets = ws
		}
	}
	if ext, ok, _ := s.store.GetSetting(r.Context(), "logo_ext"); ok && ext != "" {
		version, _, _ := s.store.GetSetting(r.Context(), "logo_updated_at")
		url := "/api/branding/logo?v=" + version
		resp.LogoURL = &url
	}
	return resp
}

func settingBool(s *Server, r *http.Request, key string, def bool) bool {
	v, ok, _ := s.store.GetSetting(r.Context(), key)
	if !ok {
		return def
	}
	return v == "true"
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.currentSettings(r))
}

type updateSettingsRequest struct {
	AppName        string    `json:"appName"`
	AccentColor    string    `json:"accentColor"`
	ShowSystem     *bool     `json:"showSystem"`
	ShowContainers *bool     `json:"showContainers"`
	Theme          *string   `json:"theme"`
	Density        *string   `json:"density"`
	RefreshSeconds *int      `json:"refreshSeconds"`
	WarnThreshold  *int      `json:"warnThreshold"`
	CritThreshold  *int      `json:"critThreshold"`
	Widgets        *[]string `json:"widgets"`
	FooterText     *string   `json:"footerText"`
}

// validateAppearance checks the optional appearance fields and returns the
// settings keys/values to persist, so nothing is written if any is invalid.
func validateAppearance(req updateSettingsRequest) (map[string]string, error) {
	out := map[string]string{}
	if req.Theme != nil {
		if !validThemes[*req.Theme] {
			return nil, fmt.Errorf("unknown theme %q", *req.Theme)
		}
		out["theme"] = *req.Theme
	}
	if req.Density != nil {
		if !validDensities[*req.Density] {
			return nil, fmt.Errorf("unknown density %q", *req.Density)
		}
		out["density"] = *req.Density
	}
	if req.RefreshSeconds != nil {
		if *req.RefreshSeconds < 2 || *req.RefreshSeconds > 300 {
			return nil, errors.New("refresh interval must be between 2 and 300 seconds")
		}
		out["refresh_seconds"] = strconv.Itoa(*req.RefreshSeconds)
	}
	warn, crit := -1, -1
	if req.WarnThreshold != nil {
		warn = *req.WarnThreshold
		out["warn_threshold"] = strconv.Itoa(warn)
	}
	if req.CritThreshold != nil {
		crit = *req.CritThreshold
		out["crit_threshold"] = strconv.Itoa(crit)
	}
	for _, t := range []int{warn, crit} {
		if t != -1 && (t < 1 || t > 100) {
			return nil, errors.New("thresholds must be between 1 and 100")
		}
	}
	if warn != -1 && crit != -1 && warn >= crit {
		return nil, errors.New("warning threshold must be below the critical threshold")
	}
	if req.Widgets != nil {
		known := map[string]bool{}
		for _, w := range validWidgets {
			known[w] = true
		}
		seen := map[string]bool{}
		for _, w := range *req.Widgets {
			if !known[w] || seen[w] {
				return nil, fmt.Errorf("unknown or duplicate widget %q", w)
			}
			seen[w] = true
		}
		b, _ := json.Marshal(*req.Widgets)
		out["overview_widgets"] = string(b)
	}
	if req.FooterText != nil {
		f := strings.TrimSpace(*req.FooterText)
		if len(f) > maxFooterLen {
			return nil, fmt.Errorf("footer text must be %d characters or fewer", maxFooterLen)
		}
		out["footer_text"] = f
	}
	return out, nil
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	appearance, err := validateAppearance(req)
	if err != nil {
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

	accent := strings.TrimSpace(req.AccentColor)
	if accent != "" {
		if !accentColorPattern.MatchString(accent) {
			writeError(w, http.StatusBadRequest, errors.New("accent color must be a 6-digit hex code like #ffb020"))
			return
		}
		if err := s.store.SetSetting(r.Context(), "accent_color", accent); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	if req.ShowSystem != nil {
		if err := s.store.SetSetting(r.Context(), "dashboard_show_system", strconv.FormatBool(*req.ShowSystem)); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if req.ShowContainers != nil {
		if err := s.store.SetSetting(r.Context(), "dashboard_show_containers", strconv.FormatBool(*req.ShowContainers)); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	for k, v := range appearance {
		if err := s.store.SetSetting(r.Context(), k, v); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
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
