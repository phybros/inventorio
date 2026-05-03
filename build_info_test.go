package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildInfoDisplayDefaults(t *testing.T) {
	info := BuildInfo{Version: "", Commit: "0123456789abcdef"}

	if got, want := info.DisplayVersion(), "dev"; got != want {
		t.Fatalf("DisplayVersion() = %q, want %q", got, want)
	}
	if got, want := info.DisplayCommit(), "0123456789ab"; got != want {
		t.Fatalf("DisplayCommit() = %q, want %q", got, want)
	}
}

func TestHealthzReturnsBuildInfo(t *testing.T) {
	oldVersion, oldCommit, oldBuildDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = "v1.2.3", "abc123", "2026-05-03T12:00:00Z"
	t.Cleanup(func() {
		Version, Commit, BuildDate = oldVersion, oldCommit, oldBuildDate
	})

	app := &App{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	app.HandleHealthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var body struct {
		Status    string `json:"status"`
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuildDate string `json:"build_date"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body.Status != "ok" || body.Version != Version || body.Commit != Commit || body.BuildDate != BuildDate {
		t.Fatalf("unexpected health response: %+v", body)
	}
}

func TestRendererIncludesBuildFooter(t *testing.T) {
	oldVersion, oldCommit, oldBuildDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = "v1.2.3", "0123456789abcdef", "2026-05-03T12:00:00Z"
	t.Cleanup(func() {
		Version, Commit, BuildDate = oldVersion, oldCommit, oldBuildDate
	})

	renderer := NewRenderer()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req = withAuthMode(req, authModeOAuth)
	rr := httptest.NewRecorder()

	renderer.RenderPage(rr, req, "auth/login", loginPageData{
		Next:             "/components",
		GitHubConfigured: true,
	})

	body := rr.Body.String()
	for _, want := range []string{"Inventorio v1.2.3", "0123456789ab"} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered body did not contain %q", want)
		}
	}
}
