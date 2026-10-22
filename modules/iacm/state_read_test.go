// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

package iacm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateFile is `terraform show -json` output with a root resource, a data
// source, a resource in a nested module, an indexed instance, a deposed
// object, and sensitive attributes.
const stateFile = `{
  "format_version": "1.0",
  "values": {
    "root_module": {
      "resources": [
        {"address": "random_pet.demo", "mode": "managed", "type": "random_pet", "name": "demo",
         "provider_name": "registry.terraform.io/hashicorp/random",
         "values": {"id": "calm-fox", "length": 2},
         "sensitive_values": {}},
        {"address": "data.aws_caller_identity.me", "mode": "data", "type": "aws_caller_identity", "name": "me",
         "provider_name": "registry.terraform.io/hashicorp/aws",
         "values": {"account_id": "123"},
         "sensitive_values": {}},
        {"address": "aws_db_instance.db", "mode": "managed", "type": "aws_db_instance", "name": "db",
         "provider_name": "registry.terraform.io/hashicorp/aws",
         "values": {"name": "main", "password": "hunter2", "tags": {"env": "prod"}, "endpoint": "db.local"},
         "sensitive_values": {"password": true, "tags": {"env": false}, "endpoint": false}},
        {"address": "random_pet.demo", "deposed_key": "abc123", "mode": "managed", "type": "random_pet", "name": "demo",
         "provider_name": "registry.terraform.io/hashicorp/random", "values": {"id": "old-fox"}}
      ],
      "child_modules": [
        {"address": "module.net",
         "resources": [
           {"address": "module.net.aws_subnet.a[0]", "mode": "managed", "type": "aws_subnet", "name": "a", "index": 0,
            "provider_name": "registry.terraform.io/hashicorp/aws", "values": {"cidr": "10.0.0.0/24"}, "sensitive_values": {}}
         ],
         "child_modules": [
           {"address": "module.net.module.dns",
            "resources": [
              {"address": "module.net.module.dns.aws_route53_zone.z", "mode": "managed", "type": "aws_route53_zone", "name": "z",
               "provider_name": "registry.terraform.io/hashicorp/aws", "values": {"name": "example.com"}, "sensitive_values": {}}
            ]}
         ]}
      ]
    }
  }
}`

func TestParseStateEntries(t *testing.T) {
	entries, err := parseStateEntries(strings.NewReader(stateFile))
	require.NoError(t, err)

	var addresses []string
	for _, e := range entries {
		addresses = append(addresses, e.res.Address)
	}
	assert.Equal(t, []string{
		"aws_db_instance.db",
		"data.aws_caller_identity.me",
		"module.net.aws_subnet.a[0]",
		"module.net.module.dns.aws_route53_zone.z",
		"random_pet.demo",
	}, addresses, "sorted by address, with nested modules and data sources, and without the deposed object")

	rows := map[string]map[string]any{}
	for _, e := range entries {
		rows[e.res.Address] = summaryRow(e)
	}
	assert.Equal(t, map[string]any{
		"address": "random_pet.demo", "type": "random_pet", "name": "demo",
		"provider": "registry.terraform.io/hashicorp/random", "module": "root", "mode": "managed",
	}, rows["random_pet.demo"])
	assert.Equal(t, "data", rows["data.aws_caller_identity.me"]["mode"])
	assert.Equal(t, "module.net", rows["module.net.aws_subnet.a[0]"]["module"])
	assert.Equal(t, "module.net.module.dns", rows["module.net.module.dns.aws_route53_zone.z"]["module"])
	assert.NotContains(t, rows["random_pet.demo"], "attributes", "list rows carry no attribute values")
}

func TestParseStateEntries_EmptyAndInvalid(t *testing.T) {
	t.Run("a state with no values has no resources", func(t *testing.T) {
		entries, err := parseStateEntries(strings.NewReader(`{"format_version":"1.0"}`))
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
	t.Run("a file that is not JSON is an error", func(t *testing.T) {
		_, err := parseStateEntries(strings.NewReader(`not json`))
		assert.ErrorContains(t, err, "reading the state file")
	})
}

func TestDetailRowMasksSensitiveAttributes(t *testing.T) {
	entries, err := parseStateEntries(strings.NewReader(stateFile))
	require.NoError(t, err)

	var db stateEntry
	for _, e := range entries {
		if e.res.Address == "aws_db_instance.db" {
			db = e
		}
	}

	attrs := detailRow(db)["attributes"].(map[string]any)

	assert.Equal(t, sensitiveValuePlaceholder, attrs["password"], "a sensitive attribute is masked")
	assert.Equal(t, "main", attrs["name"])
	assert.Equal(t, "db.local", attrs["endpoint"], "an attribute marked false is shown")
	assert.Equal(t, map[string]any{"env": "prod"}, attrs["tags"], "a nested marker with nothing sensitive shows the attribute")
	assert.Equal(t, "hunter2", db.res.AttributeValues["password"], "the parsed state itself is not modified")
}

func TestIsSensitiveMarker(t *testing.T) {
	assert.True(t, isSensitiveMarker(true))
	assert.False(t, isSensitiveMarker(false))
	assert.False(t, isSensitiveMarker(nil))
	assert.True(t, isSensitiveMarker(map[string]any{"a": false, "b": map[string]any{"c": true}}))
	assert.False(t, isSensitiveMarker(map[string]any{"a": false}))
	assert.True(t, isSensitiveMarker([]any{false, map[string]any{"x": true}}))
	assert.False(t, isSensitiveMarker([]any{}))
}
