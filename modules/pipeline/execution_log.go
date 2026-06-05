// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/harness/harness-cli/pkg/auth"
	"github.com/harness/harness-cli/pkg/cmdctx"
	"github.com/harness/harness-cli/pkg/console"
	"github.com/harness/harness-cli/pkg/format"
	"github.com/harness/harness-cli/pkg/logstream"
	"github.com/harness/harness-cli/pkg/spec"
)

const listExecutionLogsFetchFnID = "list_execution_logs_fetch"
const getPipelineLogHandlerID = "get_pipeline_log"
const pipelineLogStageCompletionID = "pipeline_log_stage_completion"
const pipelineLogStepCompletionID = "pipeline_log_step_completion"

func getPipelineLogHandler(ctx *cmdctx.Ctx) error {
	if ctx.Id == "" {
		return fmt.Errorf("missing required argument <log-key>")
	}

	if cmdctx.GetBool(ctx.FlagValues, "ui") {
		if !console.IsBothTTY() {
			return fmt.Errorf("--ui requires an interactive terminal (both stdin and stdout must be a TTY)")
		}
		stg := cmdctx.GetString(ctx.FlagValues, "stage")
		stp := cmdctx.GetString(ctx.FlagValues, "step")
		if stg != "" || stp != "" {
			return fmt.Errorf("--ui is not compatible with --stage or --step")
		}
		clean := strings.TrimRight(ctx.Id, "/")
		if strings.Count(clean, "/") > 1 {
			return fmt.Errorf("--ui requires <[pipeline/]execId> format, not a full log key")
		}
		hc := &http.Client{Timeout: 30 * time.Second}
		return RunLogViewer(execLabelFromID(ctx.Id), hc, ctx.Auth)
	}

	if cmdctx.GetBool(ctx.FlagValues, "follow") {
		hc := &http.Client{Timeout: 90 * time.Minute}
		stageFlag := cmdctx.GetString(ctx.FlagValues, "stage")
		stepFlag := cmdctx.GetString(ctx.FlagValues, "step")
		clean := strings.TrimRight(ctx.Id, "/")
		parts := strings.SplitN(clean, "/", 4)
		if len(parts) <= 3 || stageFlag != "" || stepFlag != "" {
			execId := logstream.ExecIdFromArg([]string{ctx.Id})
			return logstream.FollowMulti(ctx, hc, execId, stageFlag, stepFlag, nil)
		}
		return logstream.FollowLog(ctx, hc, clean)
	}

	a := ctx.Auth
	hc := &http.Client{Timeout: 30 * time.Second}
	fmtFlag := ctx.FormatFlags.Format

	stageFlag := cmdctx.GetString(ctx.FlagValues, "stage")
	stepFlag := cmdctx.GetString(ctx.FlagValues, "step")
	if stepFlag != "" && stageFlag == "" {
		return fmt.Errorf("--step requires --stage")
	}

	isPrefixMode := strings.HasSuffix(ctx.Id, "/") || stageFlag != ""
	id := strings.TrimRight(ctx.Id, "/")
	parts := strings.SplitN(id, "/", 4)
	if len(parts) <= 2 {
		isPrefixMode = true
	}

	w, closeW, err := format.OpenWriter(ctx.FormatFlags.OutFile)
	if err != nil {
		return err
	}
	defer closeW()

	if isPrefixMode {
		if len(parts) < 1 {
			return fmt.Errorf("prefix mode requires at least <execId>")
		}
		var execId, filterPrefix string
		if len(parts) == 1 {
			execId = parts[0]
		} else if len(parts) == 2 {
			execId = parts[1]
		} else {
			execId = strings.TrimPrefix(parts[2], "-")
			if len(parts) == 4 {
				filterPrefix = id
			}
		}
		entries, _, err := logstream.FetchLogKeys(hc, a, execId)
		if err != nil {
			return err
		}
		noHeader := cmdctx.GetBool(ctx.FlagValues, "no-header")

		type result struct {
			key  string
			body string
			err  error
		}
		var results []result
		var matchedKeys int
		for _, e := range entries {
			if filterPrefix != "" && !strings.HasPrefix(e.LogKey, filterPrefix) {
				continue
			}
			matchedKeys++
			if stageFlag != "" {
				parts := strings.SplitN(e.LogKey, "/", 6)
				if len(parts) < 4 || !strings.EqualFold(parts[3], stageFlag) {
					continue
				}
				if stepFlag != "" && (len(parts) < 5 || !strings.EqualFold(parts[4], stepFlag)) {
					continue
				}
			}
			var buf strings.Builder
			hasContent, fetchErr := logstream.FetchAndPrintLog(hc, a, e.LogKey, fmtFlag, ctx.IsPty, &buf)
			if fetchErr != nil {
				results = append(results, result{key: e.LogKey, err: fetchErr})
				continue
			}
			if !hasContent {
				continue
			}
			results = append(results, result{key: e.LogKey, body: buf.String()})
		}

		if filterPrefix != "" && matchedKeys == 0 {
			return fmt.Errorf("no log keys matched prefix %q", filterPrefix)
		}

		showHeaders := !noHeader
		for _, r := range results {
			if r.err != nil {
				if showHeaders {
					fmt.Fprintf(w, "\n== %s ==\n", r.key)
				}
				fmt.Fprintf(w, "(error: %v)\n", r.err)
				continue
			}
			if showHeaders {
				fmt.Fprintf(w, "\n== %s ==\n", r.key)
			}
			fmt.Fprint(w, r.body)
		}
		return nil
	}

	hasContent, err := logstream.FetchAndPrintLog(hc, a, ctx.Id, fmtFlag, ctx.IsPty, w)
	if err != nil {
		return err
	}
	if !hasContent {
		fmt.Fprintf(os.Stderr, "no log content for key %q\n", ctx.Id)
	}
	return nil
}

func pipelineLogStageCompletion(a *auth.ResolvedAuth, args []string, flags *pflag.FlagSet) ([]string, error) {
	execId := logstream.ExecIdFromArg(args)
	if execId == "" {
		return nil, nil
	}
	hc := &http.Client{Timeout: 30 * time.Second}
	entries, _, err := logstream.FetchLogKeys(hc, a, execId)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var stages []string
	for _, e := range entries {
		parts := strings.SplitN(e.LogKey, "/", 5)
		if len(parts) >= 4 {
			stage := parts[3]
			if !seen[stage] {
				seen[stage] = true
				stages = append(stages, stage)
			}
		}
	}
	return stages, nil
}

func pipelineLogStepCompletion(a *auth.ResolvedAuth, args []string, flags *pflag.FlagSet) ([]string, error) {
	stage, _ := flags.GetString("stage")
	if stage == "" {
		return nil, nil
	}
	execId := logstream.ExecIdFromArg(args)
	if execId == "" {
		return nil, nil
	}
	hc := &http.Client{Timeout: 30 * time.Second}
	entries, _, err := logstream.FetchLogKeys(hc, a, execId)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var steps []string
	for _, e := range entries {
		parts := strings.SplitN(e.LogKey, "/", 6)
		if len(parts) >= 5 && strings.EqualFold(parts[3], stage) {
			step := parts[4]
			if !seen[step] {
				seen[step] = true
				steps = append(steps, step)
			}
		}
	}
	return steps, nil
}

func listExecutionLogsFetchFn(ctx *cmdctx.Ctx, _ *spec.EndpointSpec, _, _ int, _ any) (*cmdctx.PageResult, error) {
	if ctx.ParentId == "" {
		return nil, fmt.Errorf("missing required argument <execution-id>")
	}
	execId := ctx.ParentId
	if i := strings.LastIndex(execId, "/"); i >= 0 {
		execId = execId[i+1:]
	}

	hc := &http.Client{Timeout: 30 * time.Second}
	entries, _, err := logstream.FetchLogKeys(hc, ctx.Auth, execId)
	if err != nil {
		return nil, err
	}

	rows := make([]any, len(entries))
	for i, e := range entries {
		rows[i] = map[string]any{"log_key": e.LogKey, "name": e.Name, "status": e.Status}
	}
	return &cmdctx.PageResult{
		Items:       rows,
		StartOffset: 0,
		Last:        true,
		HasTotal:    true,
		Total:       int64(len(rows)),
	}, nil
}
