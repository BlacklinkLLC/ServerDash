package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultFontFamily        = "modern"
	defaultBorderRadius      = "default"
	defaultBackgroundPattern = "none"
	defaultLogoHeight        = 22
	maxTextLength            = 200
	maxFooterLinks           = 5
	maxWidgetItems           = 24
)

var (
	validFontFamilies       = map[string]bool{"modern": true, "classic": true, "rounded": true, "mono": true}
	validBorderRadii        = map[string]bool{"sharp": true, "soft": true, "default": true, "round": true}
	validBackgroundPatterns = map[string]bool{"none": true, "grid": true, "dots": true, "radiant": true}
	validWidgetWidths       = map[string]bool{"full": true, "half": true, "third": true}
	footerLinkIconPattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,40}$`)

	builtinWidgetOrder = []string{"health", "gauges", "cpuChart", "memChart", "netChart", "cores", "disks", "containers", "processes"}

	builtinWidgetLabels = map[string]string{
		"health":     "Health banner",
		"gauges":     "Resource gauges",
		"cpuChart":   "CPU history chart",
		"memChart":   "Memory history chart",
		"netChart":   "Network throughput chart",
		"cores":      "Per-core CPU bars",
		"disks":      "Storage volumes",
		"containers": "Container summary",
		"processes":  "Top processes",
	}

	builtinWidgetWidths = map[string]string{
		"health": "full", "gauges": "full", "cpuChart": "half", "memChart": "half",
		"netChart": "full", "cores": "half", "disks": "half", "containers": "half", "processes": "half",
	}
)

type widgetLayout struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Width   string `json:"width,omitempty"`
}

type footerLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Icon  string `json:"icon,omitempty"`
}

type widgetCatalogItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Width    string `json:"width,omitempty"`
	PluginID string `json:"pluginId,omitempty"`
	WidgetID string `json:"widgetId,omitempty"`
}

type appearancePatch struct {
	Theme             *string
	Density           *string
	FontFamily        *string
	BorderRadius      *string
	BackgroundPattern *string
	RefreshSeconds    *float64
	WarnThreshold     *int
	CritThreshold     *int
	LogoHeight        *int
	Tagline           *string
	LoginMessage      *string
	ShowBrandBadge    *bool
	FaviconFromLogo   *bool
	FooterText        *string
	FooterLinks       *[]footerLink
	Widgets           *[]string
	WidgetLayout      *[]widgetLayout
}

type updateSettingsRequest struct {
	AppName           string          `json:"appName"`
	AccentColor       string          `json:"accentColor"`
	ShowSystem        *bool           `json:"showSystem"`
	ShowContainers    *bool           `json:"showContainers"`
	Theme             *string         `json:"theme"`
	Density           *string         `json:"density"`
	FontFamily        *string         `json:"fontFamily"`
	BorderRadius      *string         `json:"borderRadius"`
	BackgroundPattern *string         `json:"backgroundPattern"`
	RefreshSeconds    *float64        `json:"refreshSeconds"`
	WarnThreshold     *int            `json:"warnThreshold"`
	CritThreshold     *int            `json:"critThreshold"`
	LogoHeight        *int            `json:"logoHeight"`
	Tagline           *string         `json:"tagline"`
	LoginMessage      *string         `json:"loginMessage"`
	ShowBrandBadge    *bool           `json:"showBrandBadge"`
	FaviconFromLogo   *bool           `json:"faviconFromLogo"`
	FooterText        *string         `json:"footerText"`
	FooterLinks       *[]footerLink   `json:"footerLinks"`
	Widgets           *[]string       `json:"widgets"`
	WidgetLayout      *[]widgetLayout `json:"widgetLayout"`
}

type userPreferences struct {
	Theme             *string         `json:"theme,omitempty"`
	Density           *string         `json:"density,omitempty"`
	FontFamily        *string         `json:"fontFamily,omitempty"`
	BorderRadius      *string         `json:"borderRadius,omitempty"`
	BackgroundPattern *string         `json:"backgroundPattern,omitempty"`
	RefreshSeconds    *float64        `json:"refreshSeconds,omitempty"`
	WarnThreshold     *int            `json:"warnThreshold,omitempty"`
	CritThreshold     *int            `json:"critThreshold,omitempty"`
	ShowSystem        *bool           `json:"showSystem,omitempty"`
	ShowContainers    *bool           `json:"showContainers,omitempty"`
	WidgetLayout      *[]widgetLayout `json:"widgetLayout,omitempty"`
}

func (r updateSettingsRequest) appearancePatch() appearancePatch {
	return appearancePatch{
		Theme: r.Theme, Density: r.Density, FontFamily: r.FontFamily, BorderRadius: r.BorderRadius,
		BackgroundPattern: r.BackgroundPattern, RefreshSeconds: r.RefreshSeconds,
		WarnThreshold: r.WarnThreshold, CritThreshold: r.CritThreshold, LogoHeight: r.LogoHeight,
		Tagline: r.Tagline, LoginMessage: r.LoginMessage, ShowBrandBadge: r.ShowBrandBadge,
		FaviconFromLogo: r.FaviconFromLogo, FooterText: r.FooterText, FooterLinks: r.FooterLinks,
		Widgets: r.Widgets, WidgetLayout: r.WidgetLayout,
	}
}

func (p userPreferences) appearancePatch() appearancePatch {
	return appearancePatch{
		Theme: p.Theme, Density: p.Density, FontFamily: p.FontFamily, BorderRadius: p.BorderRadius,
		BackgroundPattern: p.BackgroundPattern, RefreshSeconds: p.RefreshSeconds,
		WarnThreshold: p.WarnThreshold, CritThreshold: p.CritThreshold, WidgetLayout: p.WidgetLayout,
	}
}

func pluginWidgetID(pluginID, widgetID string) string {
	return "plugin:" + pluginID + ":" + widgetID
}

func defaultFooterLinks() []footerLink {
	return []footerLink{
		{Label: "Blacklink", URL: "https://blacklink.net", Icon: "Globe"},
		{Label: "NOVA", URL: "https://nova.blacklink.net", Icon: "Brush"},
		{Label: "CDN", URL: "https://cdn.blacklink.net", Icon: "Cloud"},
	}
}

func idsToWidgetLayout(ids []string, catalog map[string]widgetCatalogItem) []widgetLayout {
	out := make([]widgetLayout, 0, len(ids))
	for _, id := range ids {
		width := "half"
		if item, ok := catalog[id]; ok && item.Width != "" {
			width = item.Width
		}
		out = append(out, widgetLayout{ID: id, Enabled: true, Width: width})
	}
	return out
}

func widgetLayoutIDs(layout []widgetLayout) []string {
	out := make([]string, 0, len(layout))
	for _, item := range layout {
		if item.Enabled {
			out = append(out, item.ID)
		}
	}
	return out
}

func (s *Server) widgetCatalog(r *http.Request) map[string]widgetCatalogItem {
	out := map[string]widgetCatalogItem{}
	for _, id := range builtinWidgetOrder {
		out[id] = widgetCatalogItem{ID: id, Label: builtinWidgetLabels[id], Width: builtinWidgetWidths[id]}
	}
	for _, p := range s.plugins.List() {
		st := s.plugins.State(r.Context(), p.ID)
		if !st.Enabled {
			continue
		}
		for _, w := range p.Widgets {
			width := "half"
			if w.Wide {
				width = "full"
			}
			id := pluginWidgetID(p.ID, w.ID)
			title := w.Title
			if title == "" {
				title = w.ID
			}
			out[id] = widgetCatalogItem{ID: id, Label: p.Name + " — " + title, Width: width, PluginID: p.ID, WidgetID: w.ID}
		}
	}
	return out
}

func sortedCatalog(catalog map[string]widgetCatalogItem) []widgetCatalogItem {
	out := make([]widgetCatalogItem, 0, len(catalog))
	for _, item := range catalog {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PluginID == out[j].PluginID {
			return out[i].Label < out[j].Label
		}
		return out[i].PluginID == ""
	})
	return out
}

func (s *Server) currentWidgetLayout(r *http.Request) []widgetLayout {
	raw, ok, _ := s.store.GetSetting(r.Context(), "widget_layout")
	if !ok || raw == "" {
		return nil
	}
	var layout []widgetLayout
	if json.Unmarshal([]byte(raw), &layout) != nil || len(layout) == 0 {
		return nil
	}
	return layout
}

func validateWidgetLayout(layout []widgetLayout, catalog map[string]widgetCatalogItem) ([]widgetLayout, error) {
	if len(layout) > maxWidgetItems {
		return nil, fmt.Errorf("dashboard can have at most %d widgets", maxWidgetItems)
	}
	clean := make([]widgetLayout, 0, len(layout))
	seen := map[string]bool{}
	for _, item := range layout {
		id := strings.TrimSpace(item.ID)
		catalogItem, ok := catalog[id]
		if !ok {
			continue
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate dashboard widget %q", id)
		}
		seen[id] = true
		width := strings.TrimSpace(item.Width)
		if width == "" {
			width = catalogItem.Width
		}
		if width == "" {
			width = "half"
		}
		if !validWidgetWidths[width] {
			return nil, fmt.Errorf("unknown widget width %q", width)
		}
		clean = append(clean, widgetLayout{ID: id, Enabled: item.Enabled, Width: width})
	}
	return clean, nil
}

func validateFooterLinks(links []footerLink) ([]footerLink, error) {
	if len(links) > maxFooterLinks {
		return nil, fmt.Errorf("at most %d footer links are allowed", maxFooterLinks)
	}
	clean := make([]footerLink, 0, len(links))
	for _, link := range links {
		label := strings.TrimSpace(link.Label)
		raw := strings.TrimSpace(link.URL)
		icon := strings.TrimSpace(link.Icon)
		if label == "" || len(label) > 40 {
			return nil, errors.New("footer link labels must be 1 to 40 characters")
		}
		if raw == "" || len(raw) > 300 {
			return nil, fmt.Errorf("footer link %q needs a URL of 300 characters or fewer", label)
		}
		if !strings.HasPrefix(raw, "/") {
			u, err := url.Parse(raw)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, fmt.Errorf("footer link %q must use http, https, or a local path", label)
			}
		}
		if icon == "" {
			icon = "Globe"
		}
		if !footerLinkIconPattern.MatchString(icon) {
			return nil, fmt.Errorf("footer link %q has an invalid icon", label)
		}
		clean = append(clean, footerLink{Label: label, URL: raw, Icon: icon})
	}
	return clean, nil
}

func validateAppearancePatch(p appearancePatch, catalog map[string]widgetCatalogItem) (map[string]string, error) {
	out := map[string]string{}
	if p.Theme != nil {
		if *p.Theme != "dark" && *p.Theme != "midnight" && *p.Theme != "light" && *p.Theme != "terminal" {
			return nil, fmt.Errorf("unknown theme %q", *p.Theme)
		}
		out["theme"] = *p.Theme
	}
	if p.Density != nil {
		if *p.Density != "comfortable" && *p.Density != "compact" {
			return nil, fmt.Errorf("unknown density %q", *p.Density)
		}
		out["density"] = *p.Density
	}
	if p.FontFamily != nil {
		if !validFontFamilies[*p.FontFamily] {
			return nil, fmt.Errorf("unknown font family %q", *p.FontFamily)
		}
		out["font_family"] = *p.FontFamily
	}
	if p.BorderRadius != nil {
		if !validBorderRadii[*p.BorderRadius] {
			return nil, fmt.Errorf("unknown border radius %q", *p.BorderRadius)
		}
		out["border_radius"] = *p.BorderRadius
	}
	if p.BackgroundPattern != nil {
		if !validBackgroundPatterns[*p.BackgroundPattern] {
			return nil, fmt.Errorf("unknown background pattern %q", *p.BackgroundPattern)
		}
		out["background_pattern"] = *p.BackgroundPattern
	}
	if p.RefreshSeconds != nil {
		if *p.RefreshSeconds < 0.1 || *p.RefreshSeconds > 300 {
			return nil, errors.New("refresh interval must be between 0.1 and 300 seconds")
		}
		out["refresh_seconds"] = strconv.FormatFloat(*p.RefreshSeconds, 'f', -1, 64)
	}
	warn, crit := -1, -1
	if p.WarnThreshold != nil {
		warn = *p.WarnThreshold
		out["warn_threshold"] = strconv.Itoa(warn)
	}
	if p.CritThreshold != nil {
		crit = *p.CritThreshold
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
	if p.LogoHeight != nil {
		if *p.LogoHeight < 16 || *p.LogoHeight > 72 {
			return nil, errors.New("logo height must be between 16 and 72 pixels")
		}
		out["logo_height"] = strconv.Itoa(*p.LogoHeight)
	}
	for _, field := range []struct {
		value *string
		key   string
		label string
	}{
		{p.Tagline, "tagline", "tagline"},
		{p.LoginMessage, "login_message", "login message"},
		{p.FooterText, "footer_text", "footer text"},
	} {
		if field.value == nil {
			continue
		}
		v := strings.TrimSpace(*field.value)
		if len(v) > maxTextLength {
			return nil, fmt.Errorf("%s must be %d characters or fewer", field.label, maxTextLength)
		}
		out[field.key] = v
	}
	for _, field := range []struct {
		value *bool
		key   string
	}{
		{p.ShowBrandBadge, "show_brand_badge"},
		{p.FaviconFromLogo, "favicon_from_logo"},
	} {
		if field.value != nil {
			out[field.key] = strconv.FormatBool(*field.value)
		}
	}
	if p.FooterLinks != nil {
		links, err := validateFooterLinks(*p.FooterLinks)
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(links)
		out["footer_links"] = string(b)
	}
	if p.Widgets != nil {
		seen := map[string]bool{}
		ids := make([]string, 0, len(*p.Widgets))
		for _, id := range *p.Widgets {
			if _, ok := catalog[id]; !ok {
				continue
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
		b, _ := json.Marshal(ids)
		layout, _ := json.Marshal(idsToWidgetLayout(ids, catalog))
		out["overview_widgets"] = string(b)
		out["widget_layout"] = string(layout)
	}
	if p.WidgetLayout != nil {
		layout, err := validateWidgetLayout(*p.WidgetLayout, catalog)
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(layout)
		ids, _ := json.Marshal(widgetLayoutIDs(layout))
		out["widget_layout"] = string(b)
		out["overview_widgets"] = string(ids)
	}
	return out, nil
}
