// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/harness/cli/v3/pkg/auth"
	"github.com/harness/cli/v3/pkg/cmdctx"
	"github.com/harness/cli/v3/pkg/console"
	"github.com/harness/cli/v3/pkg/logstream"
)

const (
	updateStateMoveHandlerID            = "update_state_move"
	updateStateReplaceProviderHandlerID = "update_state_replace_provider"
	deleteStateHandlerID                = "delete_state"
	createStateHandlerID                = "create_state"
)

// stateOperationExecution is the pipeline execution started to run a state
// operation. Nothing is persisted server-side: this execution, and its own
// terminal status, is the entire record of what ran.
type stateOperationExecution struct {
	PipelineIdentifier   string `json:"pipeline_identifier"`
	PipelineExecutionID  string `json:"pipeline_execution_id"`
	PipelineExecutionURL string `json:"pipeline_execution_url"`
}

type stateResourceSummary struct {
	Address  string `json:"address"`
	Type     string `json:"type"`
	Provider string `json:"provider"`
	Module   string `json:"module"`
	Mode     string `json:"mode"`
}

// ---- handlers ----

func updateStateMoveHandler(ctx *cmdctx.Ctx) error {
	source := cmdctx.GetString(ctx.FlagValues, "source")
	destination := cmdctx.GetString(ctx.FlagValues, "destination")
	force := cmdctx.GetBool(ctx.FlagValues, "force")

	return runStateOperation(ctx, stateOperationRequest{
		opType:           "mv",
		args:             map[string]any{"source": source, "destination": destination},
		description:      fmt.Sprintf("Move %s to %s", source, destination),
		previewAddresses: []string{source},
		force:            force,
	})
}

func updateStateReplaceProviderHandler(ctx *cmdctx.Ctx) error {
	from := cmdctx.GetString(ctx.FlagValues, "from")
	to := cmdctx.GetString(ctx.FlagValues, "to")
	force := cmdctx.GetBool(ctx.FlagValues, "force")

	return runStateOperation(ctx, stateOperationRequest{
		opType:      "replace_provider",
		args:        map[string]any{"from": from, "to": to},
		description: fmt.Sprintf("Replace provider %s with %s across the workspace's state", from, to),
		force:       force,
	})
}

func deleteStateHandler(ctx *cmdctx.Ctx) error {
	addresses := cmdctx.GetStringSlice(ctx.FlagValues, "address")
	force := cmdctx.GetBool(ctx.FlagValues, "force")

	return runStateOperation(ctx, stateOperationRequest{
		opType:           "rm",
		args:             map[string]any{"addresses": addresses},
		description:      fmt.Sprintf("Remove %d resource(s) from state: %s", len(addresses), strings.Join(addresses, ", ")),
		previewAddresses: addresses,
		force:            force,
	})
}

func createStateHandler(ctx *cmdctx.Ctx) error {
	address := cmdctx.GetString(ctx.FlagValues, "address")
	resourceID := cmdctx.GetString(ctx.FlagValues, "resource-id")
	force := cmdctx.GetBool(ctx.FlagValues, "force")

	return runStateOperation(ctx, stateOperationRequest{
		opType:      "import",
		args:        map[string]any{"address": address, "resource_id": resourceID},
		description: fmt.Sprintf("Import %s into %s", resourceID, address),
		force:       force,
	})
}

// ---- shared orchestration ----

type stateOperationRequest struct {
	opType      string
	args        map[string]any
	description string
	// previewAddresses, when non-empty, are looked up against the workspace's
	// current state snapshot and printed before the confirmation prompt - rm
	// and mv act on resources that are already tracked, so showing what is
	// there (or warning that it is not) catches a typo before it reaches the
	// pipeline. import and replace_provider have no such "already there"
	// check to make (import's whole point is a resource not yet in state).
	previewAddresses []string
	force            bool
}

func runStateOperation(ctx *cmdctx.Ctx, req stateOperationRequest) error {
	a := ctx.Auth
	workspaceID, orgID, projectID, err := resolveStateWorkspace(ctx)
	if err != nil {
		return err
	}

	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\nInterrupted. Cleaning up...")
		cancel()
	}()

	// Fail before anything runs: --ui needs a terminal, and a run started by
	// a command that cannot show it would be left unwatched.
	if err := requireTTYForUI(ctx); err != nil {
		return err
	}

	hc := &http.Client{Timeout: 60 * time.Second}

	if len(req.previewAddresses) > 0 {
		if err := previewStateAddresses(bgCtx, hc, a, orgID, projectID, workspaceID, req.previewAddresses); err != nil {
			return err
		}
	}

	fmt.Println(req.description)
	if !req.force && !console.PromptYesNo("Do you want to continue?") {
		return errors.New("canceled")
	}

	fmt.Println("Creating state operation...")
	op, err := createStateOperation(bgCtx, hc, a, orgID, projectID, workspaceID, req.opType, req.args)
	if err != nil {
		return fmt.Errorf("creating state operation: %w", err)
	}

	execURL := op.PipelineExecutionURL
	if execURL != "" {
		if !strings.HasPrefix(execURL, "http") {
			execURL = "https://" + execURL
		}
		fmt.Printf("Pipeline execution: %s\n", execURL)
	}

	if cmdctx.GetBool(ctx.FlagValues, "ui") {
		return openLogViewer(ctx, op.PipelineIdentifier, op.PipelineExecutionID)
	}

	if !cmdctx.GetBool(ctx.FlagValues, "follow") {
		fmt.Printf("State operation started. Watch the logs with: harness get execution_log %s/%s --follow\n",
			op.PipelineIdentifier, op.PipelineExecutionID)
		return nil
	}

	fmt.Println("\n=== Pipeline Execution Logs ===")
	// Same follower execute pipeline uses: nil keeps the pipeline module's base skip set.
	return logstream.FollowMulti(ctx, op.PipelineExecutionID, "", "", logstream.MultiStyleMarkers, nil)
}

// requireTTYForUI rejects --ui when there is no terminal to draw it in.
func requireTTYForUI(ctx *cmdctx.Ctx) error {
	if cmdctx.GetBool(ctx.FlagValues, "ui") && !console.IsBothTTY() {
		return errors.New("--ui requires an interactive terminal (both stdin and stdout must be a TTY)")
	}
	return nil
}

// pipelineLogHandlerID is the pipeline module's "get execution_log" workflow.
// Modules do not import each other, so the viewer is reached through the
// registry, the same way a drilldown hands off to another module's handler.
const pipelineLogHandlerID = "pipeline:get_pipeline_log"

// openLogViewer opens the interactive log viewer that
// "harness get execution_log <pipeline>/<execution-id> --ui" shows.
func openLogViewer(ctx *cmdctx.Ctx, pipelineID, executionID string) error {
	ctx.Id = pipelineID + "/" + executionID
	ctx.FlagValues = map[string]any{"ui": true}
	if err := ctx.Resolver.RunUIHandler(ctx, pipelineLogHandlerID); err != nil {
		return fmt.Errorf("opening the log viewer: %w (watch the run with: harness get execution_log %s --ui)", err, ctx.Id)
	}
	return nil
}

func resolveStateWorkspace(ctx *cmdctx.Ctx) (workspaceID, orgID, projectID string, err error) {
	a := ctx.Auth
	workspaceID = cmdctx.GetString(ctx.FlagValues, "workspace")
	orgID = a.OrgID
	projectID = a.ProjectID
	if workspaceID == "" {
		return "", "", "", errors.New("workspace is required: pass --workspace <id> or set workspace_id in .harness/workspace.yaml")
	}
	return workspaceID, orgID, projectID, nil
}

// ---- API helpers ----

func previewStateAddresses(ctx context.Context, hc *http.Client, a *auth.ResolvedAuth, org, project, workspaceID string, addresses []string) error {
	for _, addr := range addresses {
		path := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/state?search=%s",
			org, project, workspaceID, url.QueryEscape(addr))
		var resources []stateResourceSummary
		if err := doIACM(ctx, hc, a, "GET", path, nil, &resources); err != nil {
			return fmt.Errorf("checking current state for %s: %w", addr, err)
		}
		if len(resources) == 0 {
			fmt.Printf("warning: no resource matching %q found in the current state\n", addr)
			continue
		}
		for _, r := range resources {
			fmt.Printf("  %s (%s)\n", r.Address, r.Type)
		}
	}
	return nil
}

func createStateOperation(ctx context.Context, hc *http.Client, a *auth.ResolvedAuth, org, project, workspaceID, opType string, args map[string]any) (*stateOperationExecution, error) {
	path := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/state-operations", org, project, workspaceID)
	body := map[string]any{"type": opType, "args": args}
	var exec stateOperationExecution
	if err := doIACM(ctx, hc, a, "POST", path, body, &exec); err != nil {
		return nil, err
	}
	return &exec, nil
}
