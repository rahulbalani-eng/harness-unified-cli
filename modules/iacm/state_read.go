// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/harness/cli/v3/pkg/auth"
	"github.com/harness/cli/v3/pkg/cmdctx"
	"github.com/harness/cli/v3/pkg/registry"
	"github.com/harness/cli/v3/pkg/spec"
)

const (
	listStateFetchFnID = "list_state_fetch"
	getStateHandlerID  = "get_state"
	getStateItemFnID   = "get_state_item"

	// sensitiveValuePlaceholder replaces any attribute Terraform marks
	// sensitive. The state file carries the real value, so masking happens
	// here, before anything is printed.
	sensitiveValuePlaceholder = "(sensitive value)"
)

// ---- state file shape ----
//
// The workspace's latest state is stored as `terraform show -json` output
// (https://developer.hashicorp.com/terraform/internals/json-format). Only the
// fields list and get need are decoded.

type tfState struct {
	Values *struct {
		RootModule *tfModule `json:"root_module"`
	} `json:"values"`
}

type tfModule struct {
	Address      string       `json:"address"`
	Resources    []tfResource `json:"resources"`
	ChildModules []*tfModule  `json:"child_modules"`
}

type tfResource struct {
	Address         string         `json:"address"`
	Mode            string         `json:"mode"`
	Type            string         `json:"type"`
	Name            string         `json:"name"`
	ProviderName    string         `json:"provider_name"`
	DeposedKey      string         `json:"deposed_key"`
	AttributeValues map[string]any `json:"values"`
	SensitiveValues map[string]any `json:"sensitive_values"`
}

// stateEntry is one resource of the state, with its module path.
type stateEntry struct {
	res    tfResource
	module string
}

// parseStateEntries decodes a state file and returns its resources, data
// sources included, sorted by address. A deposed object is the old copy kept
// while a replacement is created, so it is not listed.
func parseStateEntries(r io.Reader) ([]stateEntry, error) {
	var st tfState
	if err := json.NewDecoder(r).Decode(&st); err != nil {
		return nil, fmt.Errorf("reading the state file: %w", err)
	}
	if st.Values == nil || st.Values.RootModule == nil {
		return nil, nil
	}

	var entries []stateEntry
	var walk func(m *tfModule)
	walk = func(m *tfModule) {
		module := m.Address
		if module == "" {
			module = "root"
		}
		for _, res := range m.Resources {
			if res.DeposedKey != "" {
				continue
			}
			entries = append(entries, stateEntry{res: res, module: module})
		}
		for _, child := range m.ChildModules {
			walk(child)
		}
	}
	walk(st.Values.RootModule)

	sort.Slice(entries, func(i, j int) bool { return entries[i].res.Address < entries[j].res.Address })
	return entries, nil
}

// summaryRow is the row `list state` shows. Attributes are not included.
func summaryRow(e stateEntry) map[string]any {
	return map[string]any{
		"address":  e.res.Address,
		"type":     e.res.Type,
		"name":     e.res.Name,
		"provider": e.res.ProviderName,
		"module":   e.module,
		"mode":     e.res.Mode,
	}
}

// detailRow is the item `get state` shows: the summary plus the attributes
// with every sensitive one masked.
func detailRow(e stateEntry) map[string]any {
	row := summaryRow(e)
	row["attributes"] = maskSensitiveAttributes(e.res.AttributeValues, e.res.SensitiveValues)
	return row
}

// maskSensitiveAttributes replaces each attribute Terraform marks sensitive
// with a fixed placeholder. A nested sensitive value masks the whole
// top-level attribute.
func maskSensitiveAttributes(attributes, sensitive map[string]any) map[string]any {
	if len(attributes) == 0 {
		return attributes
	}
	masked := make(map[string]any, len(attributes))
	for k, v := range attributes {
		masked[k] = v
	}
	for key, marker := range sensitive {
		if _, exists := masked[key]; !exists {
			continue
		}
		if isSensitiveMarker(marker) {
			masked[key] = sensitiveValuePlaceholder
		}
	}
	return masked
}

// isSensitiveMarker reports whether a sensitive_values entry marks anything
// sensitive: true, or a non-empty object or list that does.
func isSensitiveMarker(marker any) bool {
	switch v := marker.(type) {
	case bool:
		return v
	case map[string]any:
		for _, inner := range v {
			if isSensitiveMarker(inner) {
				return true
			}
		}
	case []any:
		for _, inner := range v {
			if isSensitiveMarker(inner) {
				return true
			}
		}
	}
	return false
}

// ---- reading the file ----

// loadStateEntries reads the workspace's latest state through two existing
// APIs: the workspace record, which names the stored state file, and the
// workspace data endpoint, which streams it. Nothing is parsed server-side.
// A workspace that has never been applied has no state and returns nil.
func loadStateEntries(ctx *cmdctx.Ctx) ([]stateEntry, error) {
	workspaceID, orgID, projectID, err := resolveStateWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	a := ctx.Auth
	reqCtx := ctx.Context
	if reqCtx == nil {
		reqCtx = context.Background()
	}

	ws, err := getWorkspace(reqCtx, &http.Client{Timeout: 60 * time.Second}, a, orgID, projectID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("fetching workspace: %w", err)
	}
	if ws.TerraformStateJSON == "" {
		return nil, nil
	}

	path := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/data/%s", orgID, projectID, workspaceID, ws.TerraformStateJSON)
	body, err := streamIACM(reqCtx, a, path)
	if err != nil {
		return nil, fmt.Errorf("downloading the state file: %w", err)
	}
	defer body.Close()

	return parseStateEntries(body)
}

// streamIACM returns the response body of an authenticated GET for the caller
// to read as it arrives. A state file can be large, so there is no overall
// request timeout; the caller's context bounds it.
func streamIACM(ctx context.Context, a *auth.ResolvedAuth, path string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL(a, path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", a.PATToken)
	req.Header.Set("harness-account", a.AccountID)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var apiErr iacmAPIError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr.Message)
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}
	return resp.Body, nil
}

// ---- list state ----

// listStateFetchFn is the fetch_fn of `list state`. It returns every matching
// row at once; the list driver applies --offset, --limit, and paging.
func listStateFetchFn(ctx *cmdctx.Ctx, _ *spec.EndpointSpec, _, _ int, _ any) (*cmdctx.PageResult, error) {
	entries, err := loadStateEntries(ctx)
	if err != nil {
		return nil, err
	}

	search := cmdctx.GetString(ctx.FlagValues, "search")
	rows := make([]any, 0, len(entries))
	for _, e := range entries {
		if search != "" && !strings.Contains(e.res.Address, search) {
			continue
		}
		rows = append(rows, summaryRow(e))
	}

	return &cmdctx.PageResult{
		Items:       rows,
		StartOffset: 0,
		Last:        true,
		HasTotal:    true,
		Total:       int64(len(rows)),
	}, nil
}

// ---- get state ----

// getStateItemFn finds the resource at the address in ctx.Id.
func getStateItemFn(ctx *cmdctx.Ctx) (any, error) {
	address := ctx.Id
	if address == "" {
		return nil, errors.New("missing required argument <address>")
	}
	entries, err := loadStateEntries(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.res.Address == address {
			return detailRow(e), nil
		}
	}
	return nil, fmt.Errorf("resource not found in state: %s", address)
}

func getStateHandler(ctx *cmdctx.Ctx) error {
	item, err := getStateItemFn(ctx)
	if err != nil {
		return err
	}
	return registry.RenderSingleItem(ctx, &spec.EndpointSpec{ItemExpr: "it"}, item)
}
