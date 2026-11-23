package api

import "testing"

func TestValidateAppearancePatchRejectsBadValues(t *testing.T) {
	catalog := map[string]widgetCatalogItem{"health": {ID: "health", Width: "full"}}
	bad := []appearancePatch{
		{Theme: strPtr("neon")},
		{RefreshSeconds: floatPtr(0.05)},
		{WarnThreshold: intPtr(90), CritThreshold: intPtr(80)},
		{LogoHeight: intPtr(200)},
	}
	for _, patch := range bad {
		if _, err := validateAppearancePatch(patch, catalog); err == nil {
			t.Fatalf("expected validation error for %+v", patch)
		}
	}
}

func TestValidateWidgetLayoutNormalizesWidth(t *testing.T) {
	catalog := map[string]widgetCatalogItem{"health": {ID: "health", Width: "full"}}
	layout, err := validateWidgetLayout([]widgetLayout{{ID: "health", Enabled: true}}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout) != 1 || layout[0].Width != "full" {
		t.Fatalf("unexpected layout: %+v", layout)
	}
}

func TestValidateAppearancePatchAllowsTenthSecondRefresh(t *testing.T) {
	out, err := validateAppearancePatch(appearancePatch{RefreshSeconds: floatPtr(0.1)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["refresh_seconds"] != "0.1" {
		t.Fatalf("unexpected refresh value: %q", out["refresh_seconds"])
	}
}

func TestValidateWidgetLayoutDropsStaleWidgets(t *testing.T) {
	catalog := map[string]widgetCatalogItem{"health": {ID: "health", Width: "full"}}
	layout, err := validateWidgetLayout([]widgetLayout{
		{ID: "plugin:sys-boot-time:main", Enabled: true},
		{ID: "health", Enabled: true},
	}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout) != 1 || layout[0].ID != "health" {
		t.Fatalf("unexpected layout: %+v", layout)
	}
}

func TestValidateFooterLinks(t *testing.T) {
	links, err := validateFooterLinks([]footerLink{{Label: "Docs", URL: "https://example.com", Icon: "Globe"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].URL != "https://example.com" {
		t.Fatalf("unexpected links: %+v", links)
	}
	if _, err := validateFooterLinks([]footerLink{{Label: "Bad", URL: "javascript:alert(1)"}}); err == nil {
		t.Fatal("expected javascript URL to be rejected")
	}
}

func strPtr(v string) *string     { return &v }
func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
