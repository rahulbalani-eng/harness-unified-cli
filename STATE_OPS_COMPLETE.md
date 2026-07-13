# State Operations - Implementation Complete

**Date**: July 2026  
**Branches**:
- iac-server: `IAC-7459-state-op` (latest commit: `165e3c625`)
- harness-unified-cli: `IAC-7459-state-op` (latest commit: `777c426`)

---

## All Operations Working

| Operation | CLI Command | Status |
|-----------|-------------|--------|
| List resources | `harness list state --workspace=<id>` | ✅ |
| Show resource | `harness get state <id> --address=<resource>` | ✅ |
| Move resource | `harness update state <id> --source=<old> --destination=<new>` | ✅ |
| Remove resource | `harness delete state <id> --address=<resource>` | ✅ |
| Replace provider | `harness execute state <id> --from=<old> --to=<new>` | ✅ |

---

## CLI Spec Key Fix (commit `777c426`)

Changed `body:` → `body_params:` in `pkg/spec/iacm.spec.yaml` for mutation operations.  
Without this, source/destination/reason fields were missing from request body.

---

## Quick Test

```bash
export WORKSPACE="cli6test"

# List
./bin/harness list state --workspace=$WORKSPACE

# Move
./bin/harness update state $WORKSPACE \
  --source='aws_security_group.test_sg' \
  --destination='aws_security_group.main_sg' \
  --reason="Testing rename"

# Verify
./bin/harness list state --workspace=$WORKSPACE
```

---

## For Full Testing Guide

See `STATE-OPERATIONS-TESTING.md`
