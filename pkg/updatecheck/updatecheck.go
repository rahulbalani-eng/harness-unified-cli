// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

// Package updatecheck implements background update notifications for the Harness CLI.
//
// The main process calls MaybeSpawn to potentially launch a detached subprocess, then
// calls NagIfDue to print a notice from the cache. The subprocess is launched as
// "harness --background-update-check" and calls RunBackgroundCheck to do the fetch.
package updatecheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/term"

	"github.com/harness/harness-cli/pkg/hbase"
)

const (
	// FlagName is the hidden flag that triggers the background subprocess behavior.
	FlagName = "--background-update-check"

	cacheFile       = "update-check.json"
	releasesAPIURL  = "https://app.harness.io/gateway/ng/api/harness-cli/latest-release"
	spawnInterval   = 1 * time.Hour
	checkInterval   = 24 * time.Hour
	nagInterval     = 24 * time.Hour
	httpTimeout     = 10 * time.Second
)

// cache is the on-disk cache written to ~/.harness/update-check.json.
type cache struct {
	LastSpawn     time.Time `json:"last_spawn"`
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version,omitempty"`
	LastNotified  time.Time `json:"last_notified"`
}

// MaybeSpawn checks gating conditions and, when appropriate, writes last_spawn
// and launches a detached "harness --background-update-check" subprocess.
// It is a no-op (never errors) — update checking must never break normal commands.
func MaybeSpawn() {
	if !shouldSpawn() {
		return
	}
	c := readCache()
	now := time.Now().UTC()

	if !c.LastChecked.IsZero() && now.Sub(c.LastChecked) < checkInterval {
		return // cache is fresh
	}
	if !c.LastSpawn.IsZero() && now.Sub(c.LastSpawn) < spawnInterval {
		return // already spawned recently
	}

	// Write last_spawn before spawning so concurrent invocations skip.
	c.LastSpawn = now
	_ = writeCache(c)

	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, FlagName)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	detach(cmd) // platform-specific: sets SysProcAttr to detach from the parent process group
	_ = cmd.Start()
}

// RunBackgroundCheck is the subprocess entry point. It fetches the latest version,
// updates the cache, and always exits 0.
func RunBackgroundCheck() {
	latest, err := fetchLatestVersion()
	if err != nil {
		return // silent; last_spawn already written; retry after ~1h
	}
	c := readCache()
	c.LastChecked = time.Now().UTC()
	c.LatestVersion = latest
	_ = writeCache(c)
}

// NagIfDue prints an update notice to stderr if a newer version is known and the
// nag interval has elapsed. It reads from the cache only — no network call.
func NagIfDue(currentVersion string) {
	c := readCache()
	if c.LatestVersion == "" {
		return
	}
	cur := "v" + currentVersion
	lat := "v" + c.LatestVersion
	if !semver.IsValid(cur) || !semver.IsValid(lat) {
		return
	}
	if semver.Compare(lat, cur) <= 0 {
		return // not newer
	}
	now := time.Now().UTC()
	if !c.LastNotified.IsZero() && now.Sub(c.LastNotified) < nagInterval {
		return // nagged recently
	}
	fmt.Fprintf(os.Stderr, "\nA new version of the Harness CLI is available: %s → %s\nRun: harness update\n\n", currentVersion, c.LatestVersion)
	c.LastNotified = now
	_ = writeCache(c)
}

// shouldSpawn returns false for all gating conditions that mean we skip entirely.
func shouldSpawn() bool {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}
	if hbase.IsPipelineExecution() {
		return false
	}
	if os.Getenv(hbase.EnvNoUpdateCheck) == "1" {
		return false
	}
	if isCompletionInvocation() {
		return false
	}
	return true
}

func isCompletionInvocation() bool {
	for _, arg := range os.Args[1:] {
		if arg == "__complete" || arg == "__completeNoDesc" {
			return true
		}
	}
	return false
}

func cachePath() string {
	return hbase.ExpandHomeDir(filepath.Join(hbase.HarnessHome, cacheFile))
}

func readCache() cache {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return cache{}
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return cache{}
	}
	return c
}

func writeCache(c cache) error {
	path := cachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// fetchLatestVersion calls the releases API and returns the latest semver string (without "v" prefix).
func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(releasesAPIURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if !semver.IsValid("v" + payload.Version) {
		return "", fmt.Errorf("invalid version %q from API", payload.Version)
	}
	return payload.Version, nil
}
