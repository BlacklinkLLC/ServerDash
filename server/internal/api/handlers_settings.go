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
	defaultAccent     = "#ffb020"
	maxAppNameLength  = 60
	maxLogoUploadSize = 2 << 20
)

var accentColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

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

type settingsResponse struct {
	AppName           string              `json:"appName"`
	LogoURL           *string             `json:"logoUrl,omitempty"`
	AccentColor       string              `json:"accentColor"`
	Theme             string              `json:"theme"`
	Density           string              `json:"density"`
	FontFamily        string              `json:"fontFamily"`
	BorderRadius      string              `json:"borderRadius"`
	BackgroundPattern string              `json:"backgroundPattern"`
	RefreshSeconds    float64             `json:"refreshSeconds"`
	WarnThreshold     int                 `json:"warnThreshold"`
	CritThreshold     int                 `json:"critThreshold"`
	LogoHeight        int                 `json:"logoHeight"`
	ShowSystem        bool                `json:"showSystem"`
	ShowContainers    bool                `json:"showContainers"`
	Widgets           []string            `json:"widgets"`
	WidgetLayout      []widgetLayout      `json:"widgetLayout,omitempty"`
	WidgetCatalog     []widgetCatalogItem `json:"widgetCatalog"`
	FooterText        string              `json:"footerText"`
	FooterLinks       []footerLink        `json:"footerLinks"`
	Tagline           string              `json:"tagline"`
	LoginMessage      string              `json:"loginMessage"`
	ShowBrandBadge    bool                `json:"showBrandBadge"`
	FaviconFromLogo   bool                `json:"faviconFromLogo"`
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

func settingFloat(s *Server, r *http.Request, key string, def float64) float64 {
	if v, ok, _ := s.store.GetSetting(r.Context(), key); ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func settingBool(s *Server, r *http.Request, key string, def bool) bool {
	v, ok, _ := s.store.GetSetting(r.Context(), key)
	if !ok {
		return def
	}
	return v == "true"
}

func (s *Server) currentSettings(r *http.Request) settingsResponse {
	catalog := s.widgetCatalog(r)
	widgets := append([]string(nil), builtinWidgetOrder...)
	layout := s.currentWidgetLayout(r)
	if len(layout) > 0 {
		widgets = widgetLayoutIDs(layout)
	} else if v, ok, _ := s.store.GetSetting(r.Context(), "overview_widgets"); ok {
		var legacy []string
		if json.Unmarshal([]byte(v), &legacy) == nil {
			widgets = legacy
		}
	}

	resp := settingsResponse{
		AppName:           settingString(s, r, "app_name", defaultAppName),
		AccentColor:       settingString(s, r, "accent_color", defaultAccent),
		ShowSystem:        settingBool(s, r, "dashboard_show_system", true),
		ShowContainers:    settingBool(s, r, "dashboard_show_containers", true),
		Theme:             settingString(s, r, "theme", "dark"),
		Density:           settingString(s, r, "density", "comfortable"),
		FontFamily:        settingString(s, r, "font_family", defaultFontFamily),
		BorderRadius:      settingString(s, r, "border_radius", defaultBorderRadius),
		BackgroundPattern: settingString(s, r, "background_pattern", defaultBackgroundPattern),
		RefreshSeconds:    settingFloat(s, r, "refresh_seconds", 5),
		WarnThreshold:     settingInt(s, r, "warn_threshold", 75),
		CritThreshold:     settingInt(s, r, "crit_threshold", 90),
		LogoHeight:        settingInt(s, r, "logo_height", defaultLogoHeight),
		Widgets:           widgets,
		WidgetLayout:      layout,
		WidgetCatalog:     sortedCatalog(catalog),
		FooterText:        fmt.Sprintf("ServerDash by Blacklink Enterprise. ©%d Blacklink, Inc. All Rights Reserved.", time.Now().Year()),
		FooterLinks:       defaultFooterLinks(),
		Tagline:           "Built on Blacklink Enterprise",
		LoginMessage:      settingString(s, r, "login_message", "Sign in to continue."),
		ShowBrandBadge:    true,
		FaviconFromLogo:   settingBool(s, r, "favicon_from_logo", true),
	}
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
	accent := strings.TrimSpace(req.AccentColor)
	if accent != "" && !accentColorPattern.MatchString(accent) {
		writeError(w, http.StatusBadRequest, errors.New("accent color must be a 6-digit hex code like #ffb020"))
		return
	}
	appearance, err := validateAppearancePatch(req.appearancePatch(), s.widgetCatalog(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	for _, locked := range []string{"tagline", "footer_text", "footer_links", "show_brand_badge"} {
		delete(appearance, locked)
	}

	if err := s.store.SetSetting(r.Context(), "app_name", name); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if accent != "" {
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

func (s *Server) handleServeLogo(w http.ResponseWriter, r *http.Request) {
	ext, ok, err := s.store.GetSetting(r.Context(), "logo_ext")
	if err != nil || !ok || ext == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(s.brandingDir(), "logo."+ext))
}
