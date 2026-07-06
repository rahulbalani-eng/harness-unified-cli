# Testing State Operations via CLI

**Status**: CLI needs 30-min fix before testing  
**Issue**: "state" verb not in allowed verb set

---

## 🔧 **Step 1: Fix CLI (Choose Option A or B)**

### **Option A: Use Workflow Handler (Recommended - 20 min)**

This is how `execute workspace` works. We need to create workflow handlers.

#### 1. Create workflow handlers

Create file: `modules/iacm/state.go`

```go
package iacm

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// StateList lists all resources in workspace state
func StateList(workspaceID, format string) error {
	client := GetClient() // Use your existing client
	
	org := os.Getenv("HARNESS_ORG")
	project := os.Getenv("HARNESS_PROJECT")
	account := os.Getenv("HARNESS_ACCOUNT")
	
	url := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/state/resources",
		org, project, workspaceID)
	
	resp, err := client.Get(url, map[string]string{
		"Harness-Account": account,
	})
	if err != nil {
		return fmt.Errorf("failed to list state resources: %w", err)
	}
	
	var result StateListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Format output
	switch format {
	case "json":
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
	case "addresses":
		for _, r := range result.Resources {
			fmt.Println(r.Address)
		}
	default: // table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ADDRESS\tTYPE\tMODE")
		for _, r := range result.Resources {
			fmt.Fprintf(w, "%s\t%s\t%s\n", r.Address, r.Type, r.Mode)
		}
		w.Flush()
	}
	
	return nil
}

// StateShow shows details of a specific resource
func StateShow(workspaceID, address string) error {
	client := GetClient()
	
	org := os.Getenv("HARNESS_ORG")
	project := os.Getenv("HARNESS_PROJECT")
	account := os.Getenv("HARNESS_ACCOUNT")
	
	url := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/state/resources/%s",
		org, project, workspaceID, address)
	
	resp, err := client.Get(url, map[string]string{
		"Harness-Account": account,
	})
	if err != nil {
		return fmt.Errorf("failed to show resource: %w", err)
	}
	
	var result StateShowResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Pretty print
	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(output))
	
	return nil
}

// StateMove moves/renames a resource
func StateMove(workspaceID, source, destination, reason string) error {
	client := GetClient()
	
	org := os.Getenv("HARNESS_ORG")
	project := os.Getenv("HARNESS_PROJECT")
	account := os.Getenv("HARNESS_ACCOUNT")
	
	url := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/state/resources/move",
		org, project, workspaceID)
	
	body := map[string]string{
		"source":      source,
		"destination": destination,
		"reason":      reason,
	}
	
	resp, err := client.Post(url, body, map[string]string{
		"Harness-Account": account,
	})
	if err != nil {
		return fmt.Errorf("failed to move resource: %w", err)
	}
	
	var result StateMoveResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	fmt.Printf("✅ Moved %s → %s\n", result.MovedFrom, result.MovedTo)
	fmt.Printf("New state version: %s\n", result.NewStateVersion)
	
	return nil
}

// StateRemove removes a resource from state
func StateRemove(workspaceID, address, reason string, force bool) error {
	if !force {
		fmt.Printf("⚠️  Remove %s from state? (infrastructure remains) [y/N]: ", address)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			return fmt.Errorf("cancelled")
		}
	}
	
	client := GetClient()
	
	org := os.Getenv("HARNESS_ORG")
	project := os.Getenv("HARNESS_PROJECT")
	account := os.Getenv("HARNESS_ACCOUNT")
	
	url := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/state/resources/%s",
		org, project, workspaceID, address)
	
	body := map[string]string{
		"reason": reason,
	}
	
	resp, err := client.Delete(url, body, map[string]string{
		"Harness-Account": account,
	})
	if err != nil {
		return fmt.Errorf("failed to remove resource: %w", err)
	}
	
	var result StateRemoveResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	fmt.Printf("✅ Removed %s from state\n", result.RemovedAddress)
	fmt.Printf("New state version: %s\n", result.NewStateVersion)
	
	return nil
}

// StateReplaceProvider replaces provider namespace
func StateReplaceProvider(workspaceID, from, to, reason string) error {
	client := GetClient()
	
	org := os.Getenv("HARNESS_ORG")
	project := os.Getenv("HARNESS_PROJECT")
	account := os.Getenv("HARNESS_ACCOUNT")
	
	url := fmt.Sprintf("/gateway/iacm/api/orgs/%s/projects/%s/workspaces/%s/state/providers/replace",
		org, project, workspaceID)
	
	body := map[string]string{
		"from":   from,
		"to":     to,
		"reason": reason,
	}
	
	resp, err := client.Post(url, body, map[string]string{
		"Harness-Account": account,
	})
	if err != nil {
		return fmt.Errorf("failed to replace provider: %w", err)
	}
	
	var result StateReplaceProviderResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	fmt.Printf("✅ Replaced %s → %s\n", from, to)
	fmt.Printf("Resources updated: %d\n", result.ResourcesUpdated)
	fmt.Printf("New state version: %s\n", result.NewStateVersion)
	
	return nil
}

// Response types
type StateListResponse struct {
	Resources        []StateResourceItem `json:"resources"`
	StateVersion     string              `json:"state_version"`
	TerraformVersion string              `json:"terraform_version"`
}

type StateResourceItem struct {
	Address string      `json:"address"`
	Type    string      `json:"type"`
	Name    string      `json:"name"`
	Mode    string      `json:"mode"`
	Module  *string     `json:"module,omitempty"`
	Index   interface{} `json:"index,omitempty"`
}

type StateShowResponse struct {
	Address      string                 `json:"address"`
	Resource     map[string]interface{} `json:"resource"`
	StateVersion string                 `json:"state_version"`
}

type StateMoveResponse struct {
	NewStateVersion string `json:"new_state_version"`
	ActivityID      string `json:"activity_id"`
	MovedFrom       string `json:"moved_from"`
	MovedTo         string `json:"moved_to"`
}

type StateRemoveResponse struct {
	NewStateVersion string `json:"new_state_version"`
	ActivityID      string `json:"activity_id"`
	RemovedAddress  string `json:"removed_address"`
}

type StateReplaceProviderResponse struct {
	NewStateVersion  string `json:"new_state_version"`
	ActivityID       string `json:"activity_id"`
	ResourcesUpdated int    `json:"resources_updated"`
}
```

#### 2. Update spec to use workflows

Edit `pkg/spec/iacm.spec.yaml` - replace the state commands with:

```yaml
  - command: state list
    short: List all resources in workspace terraform state
    completion_noun: workspace
    handler_type: workflow
    workflow_id: state_list
    flags:
      - name: format
        description: "Output format: table, json, or addresses"
        default: "table"

  - command: state show
    short: Show details of a specific resource in state
    completion_noun: workspace
    handler_type: workflow
    workflow_id: state_show
    flags:
      - name: address
        description: "Resource address"
        required: true

  - command: state mv
    short: Move or rename a resource in state
    completion_noun: workspace
    handler_type: workflow
    workflow_id: state_mv
    flags:
      - name: source
        description: "Source resource address"
        required: true
      - name: destination
        description: "Destination resource address"
        required: true
      - name: reason
        description: "Reason for the move"

  - command: state rm
    short: Remove a resource from state
    completion_noun: workspace
    handler_type: workflow
    workflow_id: state_rm
    flags:
      - name: address
        description: "Resource address to remove"
        required: true
      - name: reason
        description: "Reason for removal"
      - name: force
        description: "Skip confirmation"
        is_bool: true

  - command: state replace-provider
    short: Replace provider namespace in state
    completion_noun: workspace
    handler_type: workflow
    workflow_id: state_replace_provider
    flags:
      - name: from
        description: "Old provider address"
        required: true
      - name: to
        description: "New provider address"
        required: true
      - name: reason
        description: "Reason for replacement"
```

#### 3. Register workflows

In `modules/iacm/iacm.go`, register the workflows:

```go
func init() {
	// Register state operation workflows
	RegisterWorkflow("state_list", StateList)
	RegisterWorkflow("state_show", StateShow)
	RegisterWorkflow("state_mv", StateMove)
	RegisterWorkflow("state_rm", StateRemove)
	RegisterWorkflow("state_replace_provider", StateReplaceProvider)
}
```

#### 4. Build and test

```bash
go build -o bin/harness ./cmd/harness

# Test
./bin/harness state list my-workspace
./bin/harness state show my-workspace aws_instance.web
```

---

### **Option B: Simpler - Direct Implementation (10 min)**

Just remove the verb/noun structure and make them simple commands:

```yaml
  - command: state-list
    short: List all resources in workspace state
    handler_type: endpoint
    flags:
      - name: workspace
        description: "Workspace identifier"
        required: true
      - name: format
        description: "Output format"
        default: "table"
    endpoint:
      path: /gateway/iacm/api/orgs/{{auth.org}}/projects/{{auth.project}}/workspaces/{{flags.workspace}}/state/resources
      items_expr: it.resources
```

This bypasses the verb system entirely.

---

## 🧪 **Step 2: Test the CLI**

Once fixed, testing is straightforward:

### Setup Environment

```bash
# Export credentials (if not in profile)
export HARNESS_ACCOUNT="your-account"
export HARNESS_ORG="your-org"
export HARNESS_PROJECT="your-project"
export HARNESS_TOKEN="your-pat-token"

# Or use profile
export HARNESS_PROFILE="default"
```

### Test 1: List Resources

```bash
./bin/harness state list my-workspace

# With format options
./bin/harness state list my-workspace --format=json
./bin/harness state list my-workspace --format=addresses
```

**Expected output (table)**:
```
ADDRESS                     TYPE          MODE
aws_instance.web            aws_instance  managed
aws_subnet.public[0]        aws_subnet    managed
module.vpc.aws_vpc.main     aws_vpc       managed
```

**Expected output (addresses)**:
```
aws_instance.web
aws_subnet.public[0]
module.vpc.aws_vpc.main
```

### Test 2: Show Resource

```bash
./bin/harness state show my-workspace aws_instance.web
```

**Expected output**:
```json
{
  "address": "aws_instance.web",
  "resource": {
    "mode": "managed",
    "type": "aws_instance",
    "name": "web",
    "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
    "attributes": {
      "id": "i-123456",
      "ami": "ami-abc123",
      "instance_type": "t2.micro"
    }
  },
  "state_version": "abc-123"
}
```

### Test 3: Move Resource

```bash
./bin/harness state mv my-workspace \
  --source=aws_instance.web \
  --destination=aws_instance.app_server \
  --reason="Renaming for clarity"
```

**Expected output**:
```
✅ Moved aws_instance.web → aws_instance.app_server
New state version: def-456
```

### Test 4: Remove Resource

```bash
./bin/harness state rm my-workspace \
  --address=aws_instance.old \
  --reason="No longer managed"
```

**Expected output**:
```
⚠️  Remove aws_instance.old from state? (infrastructure remains) [y/N]: y
✅ Removed aws_instance.old from state
New state version: ghi-789
```

With `--force`:
```bash
./bin/harness state rm my-workspace \
  --address=aws_instance.old \
  --force
```

### Test 5: Replace Provider

```bash
./bin/harness state replace-provider my-workspace \
  --from='registry.terraform.io/-/aws' \
  --to='registry.terraform.io/hashicorp/aws' \
  --reason="Migrating to official provider"
```

**Expected output**:
```
✅ Replaced registry.terraform.io/-/aws → registry.terraform.io/hashicorp/aws
Resources updated: 5
New state version: jkl-012
```

---

## 🐛 **Troubleshooting CLI**

### "unknown command 'state'"

→ The CLI hasn't been rebuilt with the fix. Rebuild:
```bash
go build -o bin/harness ./cmd/harness
```

### "workspace not found"

→ Check workspace exists:
```bash
./bin/harness get workspace my-workspace
```

### "failed to list state resources: 401 Unauthorized"

→ Check credentials:
```bash
echo $HARNESS_TOKEN
echo $HARNESS_PROFILE

# Or use explicit flags
./bin/harness state list my-workspace --account=$ACCOUNT --org=$ORG --project=$PROJECT
```

### "verb 'state' is not in the allowed verb set"

→ You haven't applied the fix yet. Use Option A or B above.

---

## 🎯 **Quick CLI Test (After Fix)**

```bash
# 1. Build CLI
cd /Users/rahulbalani/new_cli/harness-unified-cli
go build -o bin/harness ./cmd/harness

# 2. Set profile
export HARNESS_PROFILE="default"

# 3. Test list (safe, read-only)
./bin/harness state list rahulnewcli

# 4. Test show
./bin/harness state show rahulnewcli aws_s3_bucket.example

# Success! 🎉
```

---

## 📋 **Current Status**

- ✅ Service layer works (tested via curl)
- 🚧 CLI needs 20-30 min fix (verb issue)
- ✅ Once fixed, CLI commands will work immediately

**Next Step**: Choose Option A or B above and implement it!

---

## 💡 **Recommendation**

**Use Option A (Workflow Handler)** because:
1. Follows existing pattern (`execute workspace`)
2. More control over output formatting
3. Can add confirmation prompts
4. Better error messages
5. Consistent with rest of CLI

Takes ~20 minutes to implement, then all commands work!
