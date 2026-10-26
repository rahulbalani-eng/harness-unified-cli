// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTerraformAddress(t *testing.T) {
	cases := []struct {
		name string
		res  workspaceResource
		want string
	}{
		{
			name: "root managed resource",
			res:  workspaceResource{Mode: "managed", Module: "root", Type: "random_pet", Name: "demo"},
			want: "random_pet.demo",
		},
		{
			name: "root data source",
			res:  workspaceResource{Mode: "data", Module: "root", Type: "aws_caller_identity", Name: "me"},
			want: "data.aws_caller_identity.me",
		},
		{
			name: "indexed resource in a module",
			res:  workspaceResource{Mode: "managed", Module: "module.net", Type: "aws_subnet", Name: `a[0]`},
			want: "module.net.aws_subnet.a[0]",
		},
		{
			name: "resource in a nested module",
			res:  workspaceResource{Mode: "managed", Module: "module.net.module.dns", Type: "aws_route53_zone", Name: "z"},
			want: "module.net.module.dns.aws_route53_zone.z",
		},
		{
			name: "empty module is root",
			res:  workspaceResource{Mode: "managed", Type: "aws_instance", Name: "web"},
			want: "aws_instance.web",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, terraformAddress(tc.res))
		})
	}
}

func TestSummaryRowOmitsAttributes(t *testing.T) {
	row := summaryRow(stateEntry{
		address: "random_pet.demo",
		res: workspaceResource{
			Provider:   "registry.terraform.io/hashicorp/random",
			Type:       "random_pet",
			Name:       "demo",
			Module:     "root",
			Mode:       "managed",
			Attributes: map[string]any{"id": "calm-fox"},
		},
	})

	assert.Equal(t, map[string]any{
		"address": "random_pet.demo", "type": "random_pet", "name": "demo",
		"provider": "registry.terraform.io/hashicorp/random", "module": "root", "mode": "managed",
	}, row)
}

func TestDetailRowMasksSensitiveAttributes(t *testing.T) {
	row := detailRow(stateEntry{
		address: "aws_db_instance.db",
		res: workspaceResource{
			Type: "aws_db_instance",
			Name: "db",
			Attributes: map[string]any{
				"name": "main", "password": "hunter2", "endpoint": "db.local",
			},
			SensitiveAttributes: []any{"password", 12, "missing"},
		},
	})

	attrs := row["attributes"].(map[string]any)
	assert.Equal(t, sensitiveValuePlaceholder, attrs["password"])
	assert.Equal(t, "main", attrs["name"])
	assert.Equal(t, "db.local", attrs["endpoint"])
}

func TestMaskSensitiveAttributesEmpty(t *testing.T) {
	assert.Nil(t, maskSensitiveAttributes(nil, []any{"password"}))
}
