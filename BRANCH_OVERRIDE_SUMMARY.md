# Branch Override Feature - CLI Summary

**Branch**: `IAC-7459-testing-new-cli-features`  
**Latest Commit**: `4ac3f47`  
**Date**: July 2026  
**Status**: ✅ Fully Implemented

---

## What Was Built

`--branch` flag for `harness execute workspace` command:

```bash
harness execute workspace <workspace-id> --branch=feature-branch
```

---

## Files Changed (harness-unified-cli)

| File | Change |
|------|--------|
| `pkg/spec/iacm.spec.yaml` | Added `--branch` flag as optional parameter to execute workspace command |

---

## Key Commits

| Commit | Description |
|--------|-------------|
| `6e9fb79` | Add `--branch` flag to execute workspace command |
| `5559ee2` | Add `--branch` flag to execute workspace command spec |
| `4ac3f47` | Allow optional workspace ID in execute command |

---

## CLI Spec Change

```yaml
# pkg/spec/iacm.spec.yaml
- name: execute workspace
  params:
    - name: branch
      type: string
      required: false
      description: "Git branch to execute against instead of uploading local code"
```

---

## Usage

```bash
# Execute with branch override (no local upload)
harness execute workspace my-workspace --branch=feature-branch

# Execute normally (zips and uploads local code)
harness execute workspace my-workspace

# Execute with branch and explicit workspace
harness execute workspace --workspace=my-workspace --branch=main
```

---

## Behavior Difference

| Mode | Local Code | Upload | Source |
|------|-----------|--------|--------|
| Normal | Zipped | ✅ Uploaded to server | Local directory |
| `--branch` | Not used | ❌ Skipped | Git clone in plugin |

---

## Related Branch

State operations CLI lives in `IAC-7459-state-op`.  
Backend (iac-server) changes live in `IAC-7459-testing-new-cli-features` branch of iac-server.
