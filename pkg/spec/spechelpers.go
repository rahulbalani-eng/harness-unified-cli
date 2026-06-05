// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package spec

import "sort"

// SortModules sorts metas by moduleOrder, with unlisted modules appended alphabetically.
func SortModules(metas []ModuleMeta) {
	rank := make(map[string]int, len(ModuleOrder))
	for i, name := range ModuleOrder {
		rank[name] = i
	}
	sort.SliceStable(metas, func(i, j int) bool {
		ri, iKnown := rank[metas[i].Name]
		rj, jKnown := rank[metas[j].Name]
		if iKnown && jKnown {
			return ri < rj
		}
		if iKnown {
			return true
		}
		if jKnown {
			return false
		}
		return metas[i].Name < metas[j].Name
	})
}
