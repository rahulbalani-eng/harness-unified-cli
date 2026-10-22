// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import "github.com/harness/cli/v3/pkg/registry"

// ModuleInit registers iacm workflows. Commands are declared in iacm.spec.yaml.
func ModuleInit(reg registry.ModuleRegistrar) {
	reg.RegisterWorkflow(executeWorkspaceHandlerID, executeWorkspaceHandler)
	reg.RegisterWorkflow(executeWorkspaceSynthHandlerID, executeWorkspaceSynthHandler)
	reg.RegisterWorkflow(executeWorkspaceDiffHandlerID, executeWorkspaceDiffHandler)
	reg.RegisterWorkflow(updateStateMoveHandlerID, updateStateMoveHandler)
	reg.RegisterWorkflow(updateStateReplaceProviderHandlerID, updateStateReplaceProviderHandler)
	reg.RegisterWorkflow(deleteStateHandlerID, deleteStateHandler)
	reg.RegisterWorkflow(createStateHandlerID, createStateHandler)
	reg.RegisterWorkflow(getStateHandlerID, getStateHandler)
	reg.RegisterItemFn(getStateItemFnID, getStateItemFn)
	reg.RegisterFetchFn(listStateFetchFnID, listStateFetchFn)
	reg.RegisterIdResolveFn(resolveStateScopeID, resolveStateScope)
}
