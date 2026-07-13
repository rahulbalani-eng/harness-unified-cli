# State Operations CLI Testing Guide

Complete guide for testing all terraform state operations via the Harness CLI.

## Prerequisites

```bash
# Build CLI
make build

# Set your workspace identifier
export WORKSPACE="cli6test"
```

---

## All Commands

### List resources
```bash
./bin/harness list state --workspace=$WORKSPACE
```

### Get resource details
```bash
./bin/harness get state $WORKSPACE --address='aws_instance.web' --format=json
```

### Move (rename) resource
```bash
./bin/harness update state $WORKSPACE \
  --source='aws_instance.web' \
  --destination='aws_instance.web_server' \
  --reason="Renaming"
```

### Remove resource
```bash
./bin/harness delete state $WORKSPACE \
  --address='aws_instance.db' \
  --reason="No longer managed"
```

### Replace provider
```bash
./bin/harness execute state $WORKSPACE \
  --from='registry.terraform.io/-/aws' \
  --to='registry.terraform.io/hashicorp/aws' \
  --reason="Provider migration"
```

---

## Full Test Workflow

```bash
export WORKSPACE="cli6test"

echo "=== Initial State ==="
./bin/harness list state --workspace=$WORKSPACE

echo "=== Move resource ==="
./bin/harness update state $WORKSPACE \
  --source='aws_security_group.test_sg' \
  --destination='aws_security_group.main_sg' \
  --reason="Testing move"

echo "=== After Move ==="
./bin/harness list state --workspace=$WORKSPACE

echo "=== Remove resource ==="
./bin/harness delete state $WORKSPACE \
  --address='aws_instance.db' \
  --reason="Testing remove"

echo "=== After Remove ==="
./bin/harness list state --workspace=$WORKSPACE
```

---

## Expected Output

### List state
```
 Address                     Type                Mode    
─────────────────────────────────────────────────────────
 aws_instance.web_server     aws_instance        managed 
 aws_security_group.main_sg  aws_security_group  managed 
 random_id.suffix            random_id           managed
```

### Move result
```json
{
  "activity_id": "",
  "moved_from": "aws_security_group.test_sg",
  "moved_to": "aws_security_group.main_sg",
  "new_state_version": "00890a9e-ecbe-4010-a377-a84d3ccaddae"
}
```

---

## Key Notes

- Each move/remove creates a **new immutable state version** (UUID in `new_state_version`)
- State changes are server-side only — local `terraform.tfstate` is NOT updated
- The UI may show old state (reads `state_raw`) while CLI shows current state (reads `terraform_state`)
- Sequential operations all work: each mutation reads the latest state version

---

## Known Limitations

- `activity_id` is always empty (audit trail works but ID not returned)
- No rollback command yet
- UI doesn't yet reflect state ops changes (reads different pointer)
