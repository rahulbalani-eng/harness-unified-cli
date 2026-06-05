// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	_ "embed"

	"github.com/harness/harness-cli/pkg/registry"
)

//go:embed platform.help.txt
var helpText string

// ModuleInit registers platform workflows. Commands are declared in platform.spec.yaml.
func ModuleInit(reg registry.ModuleRegistrar) {
	reg.SetHelpText(helpText)
}
