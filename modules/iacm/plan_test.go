// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestGetDefaultPipelineByOperation(t *testing.T) {
	pipelines := map[string]*defaultPipelineOverride{
		"synth": {ProjectPipeline: strPtr("projSynth")},
		"diff":  {WorkspacePipeline: strPtr("wsDiff"), ProjectPipeline: strPtr("projDiff")},
	}
	for op, want := range map[string]string{operationSynth: "projSynth", operationDiff: "wsDiff"} {
		got, err := getDefaultPipeline(pipelines, op)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", op, got, err, want)
		}
	}
	if _, err := getDefaultPipeline(pipelines, operationPlan); err == nil ||
		err.Error() != "workspace has no default pipeline configured for 'plan'" {
		t.Errorf("plan: unexpected error %v", err)
	}
}

func tarNames(t *testing.T, data []byte) []string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gr)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	sort.Strings(names)
	return names
}

func TestZipSourceCodeSkipsCDKBuildDirs(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"cdk.json", "lib/stack.ts", "node_modules/x/index.js", "cdk.out/manifest.json", ".git/HEAD"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cdk, err := zipSourceCode(root, operationSynth)
	if err != nil {
		t.Fatal(err)
	}
	if got := tarNames(t, cdk); len(got) != 2 || got[0] != "cdk.json" || got[1] != "lib/stack.ts" {
		t.Errorf("synth archive = %v", got)
	}

	plan, err := zipSourceCode(root, operationPlan)
	if err != nil {
		t.Fatal(err)
	}
	if got := tarNames(t, plan); len(got) != 4 {
		t.Errorf("plan archive should keep node_modules and cdk.out, got %v", got)
	}
}
