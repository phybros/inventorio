package main

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strings"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

func currentBuildInfo() BuildInfo {
	commit := Commit
	if commit == "" || commit == "unknown" {
		commit = vcsCommit()
	}

	return BuildInfo{
		Version:   Version,
		Commit:    commit,
		BuildDate: BuildDate,
	}
}

func (b BuildInfo) DisplayCommit() string {
	if b.Commit == "" || b.Commit == "unknown" {
		return ""
	}
	commit, dirty := strings.CutSuffix(b.Commit, "-dirty")
	if len(commit) > 12 {
		commit = commit[:12]
	}
	if dirty {
		return commit + "-dirty"
	}
	return commit
}

func vcsCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	commit := "unknown"
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if setting.Value != "" {
				commit = setting.Value
			}
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if commit == "unknown" {
		return "unknown"
	}
	if !modified {
		return commit
	}
	return commit + "-dirty"
}

func (b BuildInfo) DisplayVersion() string {
	if strings.TrimSpace(b.Version) == "" {
		return "dev"
	}
	return b.Version
}

func (app *App) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	info := currentBuildInfo()
	status := "ok"
	code := http.StatusOK

	if app.db != nil {
		if err := app.db.PingContext(r.Context()); err != nil {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
		BuildInfo
	}{
		Status:    status,
		BuildInfo: info,
	})
}
