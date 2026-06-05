// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import (
	_ "embed"

	"github.com/harness/harness-cli/pkg/registry"
)

//go:embed iacm.help.txt
var helpText string

// ModuleInit registers iacm workflows. Commands are declared in iacm.spec.yaml.
func ModuleInit(reg registry.ModuleRegistrar) {
	reg.SetHelpText(helpText)
	reg.RegisterWorkflow(executeWorkspaceHandlerID, executeWorkspaceHandler)
}
