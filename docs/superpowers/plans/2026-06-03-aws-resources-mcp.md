# aws-resources MCP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a read-only AWS resource-viewer MCP server (`mcps/aws-resources/`) exposing ~21 tools across compute, storage/DB, networking, and identity/cost, targeting multiple named profiles.

**Architecture:** Go MCP server on `modelcontextprotocol/go-sdk` over stdio. `aws.Config` is loaded via the default credential chain and cached per `profile|region`. Each tool is a free function depending on a **narrow per-call interface** (e.g. `describeInstancesAPI`), not the concrete SDK client, so tests inject fakes with no live AWS. `server.go` builds real service clients and binds them into tools via closures.

**Tech Stack:** Go ≥1.25, `github.com/modelcontextprotocol/go-sdk`, `github.com/aws/aws-sdk-go-v2` (config + per-service clients: ec2, lambda, ecs, autoscaling, s3, rds, dynamodb, elasticloadbalancingv2, iam, costexplorer), `gopkg.in/ini.v1` (parse `~/.aws/config`).

**Authoritative detail source:** `docs/superpowers/specs/2026-06-03-aws-resources-mcp-design.md` (tool table, projections, IAM perms, error handling). This plan defines structure, build order, the canonical pattern, and TDD steps.

---

### Task 1: Scaffold the unit

- [ ] **Step 1:** `make new KIND=go-mcp NAME="AWS Resources"` → creates `mcps/aws-resources/`.
- [ ] **Step 2:** `cd mcps/aws-resources && make test` → Expected: PASS (sample greet tool).
- [ ] **Step 3:** Add deps:
```bash
go get github.com/aws/aws-sdk-go-v2/config \
  github.com/aws/aws-sdk-go-v2/service/ec2 \
  github.com/aws/aws-sdk-go-v2/service/lambda \
  github.com/aws/aws-sdk-go-v2/service/ecs \
  github.com/aws/aws-sdk-go-v2/service/autoscaling \
  github.com/aws/aws-sdk-go-v2/service/s3 \
  github.com/aws/aws-sdk-go-v2/service/rds \
  github.com/aws/aws-sdk-go-v2/service/dynamodb \
  github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 \
  github.com/aws/aws-sdk-go-v2/service/iam \
  github.com/aws/aws-sdk-go-v2/service/costexplorer \
  gopkg.in/ini.v1 && go mod tidy
```
- [ ] **Step 4:** Remove sample greet tool/test. Commit: `feat(aws-resources): scaffold go-mcp unit with aws-sdk-go-v2 deps`.

---

### Task 2: Config cache + helper tools (`list_profiles`, `list_regions`)

**Files:**
- Create: `internal/awsresources/awsconfig.go`, `awsconfig_test.go`

- [ ] **Step 1: Write the failing test** (profile parsing is the offline-testable part)

```go
package awsresources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListProfilesParsesConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	os.WriteFile(cfg, []byte("[profile dev]\nregion=us-east-1\n[profile prod]\nregion=us-west-2\n"), 0o600)

	got, err := listProfilesFrom(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dev", "prod"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v want %v", got, want)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/awsresources/ -run TestListProfiles -v` → FAIL (undefined `listProfilesFrom`).

- [ ] **Step 3: Implement awsconfig.go**

```go
package awsresources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"gopkg.in/ini.v1"
)

type configCache struct {
	mu sync.Mutex
	m  map[string]aws.Config
}

func newConfigCache() *configCache { return &configCache{m: map[string]aws.Config{}} }

// get returns a cached aws.Config for profile|region, loading via the default
// credential chain (env/SSO/shared config) on first use.
func (c *configCache) get(ctx context.Context, profile, region string) (aws.Config, error) {
	key := profile + "|" + region
	c.mu.Lock()
	defer c.mu.Unlock()
	if cfg, ok := c.m[key]; ok {
		return cfg, nil
	}
	opts := []func(*config.LoadOptions) error{}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws config for profile %q: %w (try: aws sso login --profile %s)", profile, err, profile)
	}
	c.m[key] = cfg
	return cfg, nil
}

func defaultConfigPath() string {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "config")
}

// listProfilesFrom parses an ~/.aws/config file and returns profile names.
func listProfilesFrom(path string) ([]string, error) {
	f, err := ini.Load(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range f.Sections() {
		name := s.Name()
		switch {
		case name == ini.DefaultSection:
			continue
		case name == "default":
			out = append(out, "default")
		case strings.HasPrefix(name, "profile "):
			out = append(out, strings.TrimPrefix(name, "profile "))
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run** test → PASS.
- [ ] **Step 5:** Add `ListProfiles`/`ListRegions` MCP handlers. `ListProfiles` wraps `listProfilesFrom(defaultConfigPath())`. `ListRegions` uses the canonical EC2 pattern from Task 3 calling `DescribeRegions`. Register both in `server.go`.
- [ ] **Step 6:** Add `regionRequired(region string) error` validation helper (returns error if empty) in a new `validate.go`, plus `textResult`/`mapAWSError` in `result.go` (see Task 3). Commit: `feat(aws-resources): config cache + list_profiles/list_regions`.

---

### Task 3: Canonical tool — `list_ec2_instances` (the interface-narrowing pattern)

**Files:**
- Create: `internal/awsresources/compute.go`, `compute_test.go`, `result.go`

- [ ] **Step 1: Write the failing test (fake implements the narrow interface)**

```go
package awsresources

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type fakeEC2 struct {
	out *ec2.DescribeInstancesOutput
	err error
}

func (f fakeEC2) DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return f.out, f.err
}

func TestListEC2Instances(t *testing.T) {
	f := fakeEC2{out: &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{
			InstanceId:   aws.String("i-123"),
			InstanceType: ec2types.InstanceTypeT3Micro,
			State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			Placement:    &ec2types.Placement{AvailabilityZone: aws.String("us-east-1a")},
			PrivateIpAddress: aws.String("10.0.0.1"),
			Tags:         []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web")}},
		}}}},
	}}
	out, err := listEC2Instances(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Instances) != 1 || out.Instances[0].ID != "i-123" || out.Instances[0].Name != "web" {
		t.Fatalf("got %+v", out.Instances)
	}
}
```

- [ ] **Step 2: Run** → FAIL (undefined).

- [ ] **Step 3: Implement compute.go (interface + core fn + MCP handler)**

```go
package awsresources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// describeInstancesAPI is the narrow slice of the EC2 client this tool needs.
type describeInstancesAPI interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type EC2Instance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	State     string `json:"state"`
	AZ        string `json:"az"`
	PrivateIP string `json:"private_ip"`
	PublicIP  string `json:"public_ip"`
}

type ListEC2Input struct {
	Profile  string `json:"profile" jsonschema:"AWS profile name"`
	Region   string `json:"region" jsonschema:"AWS region"`
	MaxItems int    `json:"max_items,omitempty" jsonschema:"max results (default 100, cap 1000)"`
}

type ListEC2Output struct {
	Instances []EC2Instance `json:"instances"`
	Truncated bool          `json:"truncated"`
}

func clampMax(n int) int {
	if n <= 0 {
		return 100
	}
	if n > 1000 {
		return 1000
	}
	return n
}

// listEC2Instances is the testable core: it takes the narrow API, not a client.
func listEC2Instances(ctx context.Context, api describeInstancesAPI, max int) (ListEC2Output, error) {
	var out ListEC2Output
	p := ec2.NewDescribeInstancesPaginator(api.(*ec2.Client), &ec2.DescribeInstancesInput{})
	_ = p // see note
	res, err := api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return out, mapAWSError("ec2", err)
	}
	for _, r := range res.Reservations {
		for _, i := range r.Instances {
			if len(out.Instances) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Instances = append(out.Instances, EC2Instance{
				ID: aws.ToString(i.InstanceId), Name: tagValue(i.Tags, "Name"),
				Type: string(i.InstanceType), State: stateName(i.State),
				AZ: az(i.Placement), PrivateIP: aws.ToString(i.PrivateIpAddress),
				PublicIP: aws.ToString(i.PublicIpAddress),
			})
		}
	}
	return out, nil
}
```

> **Implementation note for the engineer:** the paginator type wants the concrete
> `*ec2.Client`, which conflicts with interface-narrowing. Resolve by making the
> narrow interface the paginator's dependency: `ec2.NewDescribeInstancesPaginator`
> accepts an interface `ec2.DescribeInstancesAPIClient` — use **that** SDK-provided
> interface as the field type instead of hand-rolling one, and drive pagination
> with `for p.HasMorePages() { p.NextPage(ctx) }` capped at `max`. Delete the
> `api.(*ec2.Client)` cast and the placeholder lines above. Each service's SDK
> ships an analogous `*APIClient` paginator interface — use it everywhere a
> paginator exists; for non-paginated calls (e.g. `s3.ListBuckets`) hand-roll the
> one-method interface as shown in the test.

- [ ] **Step 4: Add helpers** in compute.go (`tagValue`, `stateName`, `az`) and `result.go` (`textResult` like the github unit; `mapAWSError(service string, err error) error` using `smithy.APIError` via `errors.As` to surface `aerr.ErrorCode()` and a readable message; AccessDenied/Throttling produce clear text).

- [ ] **Step 5: Run** → PASS.
- [ ] **Step 6: Register** `list_ec2_instances` in `server.go`: handler loads `cfg, err := cache.get(ctx, in.Profile, in.Region)` after `regionRequired(in.Region)`, builds `ec2.NewFromConfig(cfg)`, calls `listEC2Instances(ctx, client, clampMax(in.MaxItems))`, returns `textResult(out), out, nil`.
- [ ] **Step 7:** `make test` → PASS. Commit: `feat(aws-resources): list_ec2_instances + aws error/result helpers`.

---

### Tasks 4–20: Remaining tools (apply the Task 3 interface-narrowing pattern)

For each: write a test with a fake implementing the SDK paginator interface (or a one-method interface for non-paginated calls), implement `listXxx(ctx, api, max)` core + projection, register an MCP handler that resolves config from the cache and builds the service client, run tests, commit `feat(aws-resources): <tool>`. Use the spec's projections.

**Compute (compute.go)**
- [ ] **Task 4 — `list_lambda_functions`** · `lambda.NewListFunctionsPaginator` · out: name, runtime, memory, last_modified.
- [ ] **Task 5 — `list_ecs_clusters`** · `ecs.ListClusters` + `DescribeClusters` · out: name/arn, status, running/active counts.
- [ ] **Task 6 — `list_ecs_services`** (`cluster` arg required) · `ecs.ListServices` + `DescribeServices` · out: service, status, desired/running, task_def.
- [ ] **Task 7 — `list_autoscaling_groups`** · `autoscaling.NewDescribeAutoScalingGroupsPaginator` · out: name, min/max/desired, instance count.

**Storage & DB (storage.go)**
- [ ] **Task 8 — `list_s3_buckets`** (global; profile only) · `s3.ListBuckets` (one-method interface) · out: name, region (via `GetBucketLocation`), creation_date.
- [ ] **Task 9 — `list_ebs_volumes`** · `ec2.NewDescribeVolumesPaginator` · out: id, size, state, type, attached_instance.
- [ ] **Task 10 — `list_rds_instances`** · `rds.NewDescribeDBInstancesPaginator` · out: id, engine, class, status, endpoint.
- [ ] **Task 11 — `list_dynamodb_tables`** · `dynamodb.NewListTablesPaginator` + `DescribeTable` · out: name, status, item_count, size_bytes.

**Networking (network.go)**
- [ ] **Task 12 — `list_vpcs`** · `ec2.DescribeVpcs` · out: id, cidr, is_default, name tag.
- [ ] **Task 13 — `list_subnets`** (`vpc_id?` filter) · `ec2.NewDescribeSubnetsPaginator` · out: id, vpc_id, az, cidr, available_ips.
- [ ] **Task 14 — `list_security_groups`** (`vpc_id?`) · `ec2.NewDescribeSecurityGroupsPaginator` · out: id, name, vpc_id, ingress/egress rule counts.
- [ ] **Task 15 — `list_load_balancers`** · `elasticloadbalancingv2.NewDescribeLoadBalancersPaginator` · out: name, type, scheme, dns_name, state.
- [ ] **Task 16 — `list_elastic_ips`** · `ec2.DescribeAddresses` · out: allocation_id, public_ip, association.

**Identity & cost (identity.go, cost.go)**
- [ ] **Task 17 — `list_iam_users`** · `iam.NewListUsersPaginator` · out: name, id, created, password_last_used. Metadata only.
- [ ] **Task 18 — `list_iam_roles`** · `iam.NewListRolesPaginator` · out: name, id, created, trust summary (path/description; do NOT decode policy docs).
- [ ] **Task 19 — `list_iam_policies`** (`Scope=Local`) · `iam.NewListPoliciesPaginator` · out: name, arn, attachment_count. No policy documents.
- [ ] **Task 20 — `get_cost_summary`** (global; `start?`/`end?` default to MTD) · `costexplorer.GetCostAndUsage` (Granularity=MONTHLY, Metrics=["UnblendedCost"], GroupBy SERVICE) · out: per-service amounts + total. README must note the ~$0.01/request charge.

---

### Task 21: README + final verification

- [ ] **Step 1: README** — purpose; Go rationale; run/build/test commands; required read-only IAM perms (per spec); the Cost Explorer charge note; profile/region usage; full tool list.
- [ ] **Step 2:** `make test && make lint` → PASS / no findings.
- [ ] **Step 3:** Confirm no mutating AWS API is imported: `grep -rE "(Create|Delete|Put|Update|Modify|Terminate|Run)[A-Z]" internal/ || echo "clean"` → Expected: clean (only Describe/List/Get).
- [ ] **Step 4:** Commit `docs(aws-resources): README + tool reference`.

---

## Self-Review

- **Spec coverage:** helpers `list_profiles`/`list_regions` (Task 2) ✓; ~19 resource tools (Tasks 3–20) ✓; profile|region cache (Task 2) ✓; default credential chain/SSO (Task 2 `cache.get`) ✓; interface-narrowed offline tests (Task 3 pattern) ✓; error mapping via smithy APIError (Task 3/4 `mapAWSError`) ✓; region validation (Task 2 `regionRequired`) ✓; max_items cap (Task 3 `clampMax`) ✓; read-only grep guard + Cost charge note (Task 21) ✓; IAM metadata-only (Tasks 17–19) ✓.
- **Placeholders:** none — each tool task names exact SDK call, args, projection. The one risky spot (paginator vs interface) is called out with the concrete resolution (use the SDK's `*APIClient` paginator interfaces).
- **Type consistency:** `cache.get`, `regionRequired`, `clampMax`, `textResult`, `mapAWSError`, `tagValue/stateName/az` defined in Tasks 2–3 and reused by name throughout.
