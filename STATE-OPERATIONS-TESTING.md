# State Operations CLI Testing Guide

Complete guide for testing all terraform state operations via the Harness CLI.

## Prerequisites

```bash
# Set your workspace identifier
export WORKSPACE="rahul_CLI_2"

# Make sure you have the latest CLI build
go build -o bin/harness ./cmd/harness
```

## 1. List Resources

See all resources in the workspace state:

```bash
./bin/harness list state --workspace=$WORKSPACE

# With JSON format for scripting
./bin/harness list state --workspace=$WORKSPACE --format=json

# Count resources by type
./bin/harness list state --workspace=$WORKSPACE --format=json | \
  jq -r '.[].type' | sort | uniq -c
```

**Expected output:**
```
 Address                     Type                Mode    
─────────────────────────────────────────────────────────
 aws_instance.db             aws_instance        managed 
 aws_instance.web            aws_instance        managed 
 aws_security_group.test_sg  aws_security_group  managed
```

## 2. Show Resource Details

Get full details of a specific resource:

```bash
# Basic resource
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json

# Extract specific fields with jq
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq '{
    address, 
    type: .resource.type, 
    id: .resource.attributes.id, 
    instance_type: .resource.attributes.instance_type,
    ami: .resource.attributes.ami,
    tags: .resource.attributes.tags
  }'

# Show security group
./bin/harness get state $WORKSPACE --address='aws_security_group.test_sg' --format=json

# Show dependencies
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq '{address, dependencies: .resource.dependencies}'

# Indexed resource (if using count or for_each)
./bin/harness get state $WORKSPACE --address='aws_instance.web[0]' --format=json
```

**Example output:**
```json
{
  "address": "aws_instance.web",
  "type": "aws_instance",
  "id": "i-02c0c789147f3e76f",
  "instance_type": "t2.micro",
  "ami": "ami-0c02fb55956c7d316",
  "tags": {
    "Env": "dev",
    "Name": "iacm-state-test"
  }
}
```

## 3. Move/Rename Resource

Rename a resource in state (infrastructure remains unchanged):

**Use Case**: You refactored terraform code and renamed a resource - need to update state to match.

```bash
# Example: Renamed aws_instance.old_web to aws_instance.web in terraform code
./bin/harness update state $WORKSPACE \
  --source='aws_instance.old_web' \
  --destination='aws_instance.web' \
  --reason="Renamed resource for clarity"

# Verify the rename
./bin/harness list state --workspace=$WORKSPACE | grep aws_instance

# Check new state version was created
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq '.state_version'
```

**Note**: This is **safe** - only updates state metadata, infrastructure is not modified.

## 4. Remove Resource

Remove a resource from state (infrastructure remains running):

**Use Case**: Infrastructure was manually deleted outside terraform, need to stop tracking it.

```bash
# WARNING: Only use with resources that are already deleted or you want to orphan
./bin/harness delete state $WORKSPACE \
  --address='aws_instance.old_server' \
  --reason="Server was manually terminated, removing from state"

# Verify it's gone from state
./bin/harness list state --workspace=$WORKSPACE | grep old_server
```

**⚠️ Warning**: Infrastructure remains running but terraform no longer manages it (orphaned resource).

## 5. Replace Provider

Migrate provider namespace (e.g., legacy `-` format to hashicorp):

**Use Case**: Upgrading terraform to use official provider registry format.

```bash
# Check current providers in state
./bin/harness list state --workspace=$WORKSPACE --format=json | \
  jq -r '.[] | .address' | head -3 | while read addr; do
    ./bin/harness get state $WORKSPACE --address="$addr" --format=json | \
      jq -r '.resource.provider'
  done | sort -u

# Replace provider namespace
./bin/harness execute state $WORKSPACE \
  --from='registry.terraform.io/-/aws' \
  --to='registry.terraform.io/hashicorp/aws' \
  --reason="Migrating to official hashicorp provider namespace"

# Expected output
# {
#   "new_state_version": "uuid",
#   "resources_updated": 47
# }
```

## Complete End-to-End Test

Run this full workflow to validate all operations:

```bash
#!/bin/bash
set -e

export WORKSPACE="rahul_CLI_2"

echo "=== Test 1: List all resources ==="
./bin/harness list state --workspace=$WORKSPACE
RESOURCE_COUNT=$(./bin/harness list state --workspace=$WORKSPACE --format=json | jq 'length')
echo "✓ Found $RESOURCE_COUNT resources"

echo -e "\n=== Test 2: Get resource details ==="
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq '{address, type: .resource.type, instance_id: .resource.attributes.id}'
echo "✓ Successfully retrieved resource details"

echo -e "\n=== Test 3: Check state version ==="
STATE_VERSION=$(./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | jq -r '.state_version')
echo "Current state version: $STATE_VERSION"
echo "✓ State version retrieved"

echo -e "\n=== Test 4: Verify resource attributes ==="
INSTANCE_ID=$(./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq -r '.resource.attributes.id')
if [ -n "$INSTANCE_ID" ]; then
  echo "✓ Instance ID: $INSTANCE_ID"
else
  echo "✗ Failed to get instance ID"
  exit 1
fi

echo -e "\n=== Test 5: Check dependencies ==="
DEPS=$(./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq -r '.resource.dependencies[]?')
if [ -n "$DEPS" ]; then
  echo "✓ Dependencies found: $DEPS"
else
  echo "ℹ️  No dependencies"
fi

echo -e "\n=== All tests passed! ✅ ==="
```

## Testing Mutation Operations Safely

To test move/remove operations without affecting production:

### Option 1: Create Test Resources

Add to your terraform code:
```hcl
resource "null_resource" "test_move_src" {
  triggers = {
    test = "move operation"
  }
}

resource "null_resource" "test_remove" {
  triggers = {
    test = "remove operation"
  }
}
```

Apply to workspace, then test:
```bash
# Test move
./bin/harness update state $WORKSPACE \
  --source='null_resource.test_move_src' \
  --destination='null_resource.test_move_dst' \
  --reason="Testing move operation"

# Test remove
./bin/harness delete state $WORKSPACE \
  --address='null_resource.test_remove' \
  --reason="Testing remove operation"
```

### Option 2: Use Dedicated Test Workspace

Create a separate workspace just for testing state operations.

## Troubleshooting

### Resource not found (404)

Check the exact address format:
```bash
./bin/harness list state --workspace=$WORKSPACE --format=json | jq -r '.[].address' | sort
```

### "missing from body" Error

Update to latest CLI:
```bash
cd harness-unified-cli
git pull origin IAC-7459-state-op
go build -o bin/harness ./cmd/harness
```

### URL Encoding for Special Characters

The CLI automatically handles URL encoding for brackets in indexed resources:
```bash
# Both work (CLI handles encoding)
./bin/harness get state $WORKSPACE --address='aws_instance.web[0]' --format=json
./bin/harness get state $WORKSPACE --address="aws_instance.web[0]" --format=json
```

### Empty Table Output

For `get state`, always use `--format=json` or `--format=yaml`:
```bash
# ✗ Bad (table format doesn't work for nested data)
./bin/harness get state $WORKSPACE --address='aws_instance.web'

# ✓ Good (JSON format shows all details)
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json
```

## Safety & Best Practices

### ✅ Safe Operations
- **List**: Read-only, always safe
- **Show**: Read-only, always safe
- **Move**: Safe - only renames in state, infrastructure unchanged

### ⚠️ Use with Caution
- **Remove**: Creates orphaned infrastructure (terraform no longer manages it)
- **Replace provider**: Safe if providers are compatible, but verify carefully

### 🛡️ Safety Features
- ✅ All mutations create new state versions (old versions preserved)
- ✅ All operations logged in workspace activity timeline  
- ✅ Operations blocked if workspace is locked (during terraform runs)
- ✅ RBAC enforced (`workspace_edit` permission required for mutations)
- ✅ Immutable state - old versions never modified

## Real-World Use Cases

### Use Case 1: Refactoring Terraform Code

**Scenario**: Renamed `aws_instance.server` to `aws_instance.web_server` in code.

```bash
# Before: State shows old name, terraform plan wants to destroy+recreate
terraform plan
# Plan: 1 to destroy, 1 to create

# Fix: Rename in state to match code
./bin/harness update state $WORKSPACE \
  --source='aws_instance.server' \
  --destination='aws_instance.web_server' \
  --reason="Renamed resource to match new code structure"

# After: State matches code, no changes needed
terraform plan
# No changes. Your infrastructure matches the configuration.
```

### Use Case 2: Removing Manually Deleted Resources

**Scenario**: Server was terminated in AWS console, terraform still tracking it.

```bash
# Check state shows the resource
./bin/harness get state $WORKSPACE --address='aws_instance.old_server' --format=json

# Remove from state (since it's already gone from AWS)
./bin/harness delete state $WORKSPACE \
  --address='aws_instance.old_server' \
  --reason="Server was manually terminated in AWS console"

# terraform plan no longer tries to recreate it
```

### Use Case 3: Provider Migration

**Scenario**: Upgrading terraform version, need to use official provider format.

```bash
# Check current provider format
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq -r '.resource.provider'
# Output: "registry.terraform.io/-/aws"

# Migrate all resources to new format
./bin/harness execute state $WORKSPACE \
  --from='registry.terraform.io/-/aws' \
  --to='registry.terraform.io/hashicorp/aws' \
  --reason="Provider migration for terraform 1.9+ compatibility"

# Verify migration
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json | \
  jq -r '.resource.provider'
# Output: "registry.terraform.io/hashicorp/aws"
```

## API Endpoints

If you prefer using curl directly:

```bash
# List resources
curl "https://app.harness.io/gateway/iacm/api/orgs/default/projects/Testim/workspaces/$WORKSPACE/state/resources" \
  -H "Harness-Account: $ACCOUNT" \
  -H "x-api-key: $TOKEN"

# Show resource
curl "https://app.harness.io/gateway/iacm/api/orgs/default/projects/Testim/workspaces/$WORKSPACE/state/resources/aws_instance.web" \
  -H "Harness-Account: $ACCOUNT" \
  -H "x-api-key: $TOKEN"
```

## Getting Help

```bash
# Command help
./bin/harness list state --help
./bin/harness get state --help
./bin/harness update state --help
./bin/harness delete state --help
./bin/harness execute state --help

# List all state commands
./bin/harness --help | grep state
```

---

**Documentation**: See `TECH-SPEC-Branch-and-State-Operations.md` for full technical details.
