// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/harness/cli/v3/pkg/cmdctx"
	"github.com/harness/cli/v3/pkg/registry"
	"github.com/harness/cli/v3/pkg/spec"
)

const (
	listStateFetchFnID = "list_state_fetch"
	getStateHandlerID  = "get_state"
	getStateItemFnID   = "get_state_item"

	// sensitiveValuePlaceholder replaces any attribute the resources table
	// marks sensitive, before anything is printed.
	sensitiveValuePlaceholder = "(sensitive value)"
)

// workspaceResource is one row from list resources. Managed resources and
// data sources share this shape; outputs are a different array and are not
// listed here.
type workspaceResource struct {
	Provider            string         `json:"provider"`
	Type                string         `json:"type"`
	Name                string         `json:"name"`
	Module              string         `json:"module"`
	Mode                string         `json:"mode"`
	Attributes          map[string]any `json:"attributes"`
	SensitiveAttributes []any          `json:"sensitive_attributes"`
}

// resourcesList is the list-resources response. The server reads the
// resources table when IACM_USE_NEW_RESOURCES_TABLE is on, and the stored
// state snapshot otherwise.
type resourcesList struct {
	Resources   []workspaceResource `json:"resources"`
	DataSources []workspaceResource `json:"data_sources"`
}

// stateEntry is one listed resource with the Terraform address rebuilt from
// its module, mode, type, and name. The resources table does not store the
// address itself.
type stateEntry struct {
	res     workspaceResource
	address string
}

// terraformAddress rebuilds the address Terraform prints. A data source is
// prefixed with "data.", and a non-root module is prefixed with its path.
// The stored name already includes a count or for_each index.
func terraformAddress(res workspaceResource) string {
	local := res.Type + "." + res.Name
	if res.Mode == "data" {
		local = "data." + local
	}
	if res.Module == "" || res.Module == "root" {
		return local
	}
	return res.Module + "." + local
}

// summaryRow is the row `list state` shows. Attributes are not included.
func summaryRow(e stateEntry) map[string]any {
	return map[string]any{
		"address":  e.address,
		"type":     e.res.Type,
		"name":     e.res.Name,
		"provider": e.res.Provider,
		"module":   e.res.Module,
		"mode":     e.res.Mode,
	}
}

// detailRow is the item `get state` shows: the summary plus the attributes,
// with every sensitive one masked.
func detailRow(e stateEntry) map[string]any {
	row := summaryRow(e)
	row["attributes"] = maskSensitiveAttributes(e.res.Attributes, e.res.SensitiveAttributes)
	return row
}

// maskSensitiveAttributes replaces each attribute named in sensitive with a
// fixed placeholder. The resources table records those names as a list of
// top-level keys.
func maskSensitiveAttributes(attributes map[string]any, sensitive []any) map[string]any {
	if len(attributes) == 0 {
		return attributes
	}
	masked := make(map[string]any, len(attributes))
	for k, v := range attributes {
		masked[k] = v
	}
	for _, key := range sensitive {
		name, ok := key.(string)
		if !ok {
			continue
		}
		if _, exists := masked[name]; exists {
			masked[name] = sensitiveValuePlaceholder
		}
	}
	return masked
}

// loadStateEntries lists the workspace's resources. Both `list state` and
// `get state` read this response instead of downloading the state file.
func loadStateEntries(ctx *cmdctx.Ctx) ([]stateEntry, error) {
	workspaceID, orgID, projectID, err := resolveStateWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	reqCtx := ctx.Context
	if reqCtx == nil {
		reqCtx = context.Background()
	}

	path := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/resources", orgID, projectID, workspaceID)
	var listed resourcesList
	if err := doIACM(reqCtx, &http.Client{Timeout: 60 * time.Second}, ctx.Auth, http.MethodGet, path, nil, &listed); err != nil {
		return nil, fmt.Errorf("listing workspace resources: %w", err)
	}

	entries := make([]stateEntry, 0, len(listed.Resources)+len(listed.DataSources))
	for _, res := range listed.Resources {
		entries = append(entries, stateEntry{res: res, address: terraformAddress(res)})
	}
	for _, res := range listed.DataSources {
		entries = append(entries, stateEntry{res: res, address: terraformAddress(res)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].address < entries[j].address })
	return entries, nil
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
		if search != "" && !strings.Contains(e.address, search) {
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
		if e.address == address {
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
