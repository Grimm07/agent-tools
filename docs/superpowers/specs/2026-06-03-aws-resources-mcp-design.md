# Design: `aws-resources` MCP server

**Date:** 2026-06-03
**Status:** Approved (design)
**Unit:** `mcps/aws-resources/`

## Purpose

A read-only Model Context Protocol server that lets an agent view live AWS
resources across the owner's accounts: compute, storage & databases, networking,
and identity & cost. Scaffolded from the `go-mcp` copier kind.

## Language choice

**Go**, consistent with the other MCP units: single static binary, official
`aws-sdk-go-v2`, long-lived stdio process. Rationale recorded in the unit README.

## Scope

In scope (all **read-only** — only `Describe*` / `List*` / `Get*` APIs):

- Compute, Storage & DB, Networking, Identity & cost (tool list below).

Out of scope: any create/modify/delete; reading secret material (IAM policy
documents, access keys, Secrets Manager values, SSM SecureString); CloudTrail
event history; multi-account org traversal via assume-role (profiles only — see
below).

## Authentication & account targeting

- Account model: **multiple named profiles** from `~/.aws/config` (SSO-friendly).
- Regional tools take `profile` + `region`. Global services (S3 bucket list, IAM,
  Cost Explorer) take `profile` only.
- Credentials resolve through the default chain:
  `config.LoadDefaultConfig(ctx, WithSharedConfigProfile(profile), WithRegion(region))`.
  Nothing is hardcoded; SSO profiles work transparently.
- `aws.Config` is built once and **cached, keyed by `profile|region`**, to avoid
  repeated SSO resolution within a session.
- Helper tools: `list_profiles` (parse `~/.aws/config` for profile names),
  `list_regions` (EC2 `DescribeRegions`).
- Invalid/expired credentials → actionable tool error (e.g. "run
  `aws sso login --profile X`").

## Tools (~21, read-only)

**Helpers**

| Tool | Input | Output |
|------|-------|--------|
| `list_profiles` | — | profile names from shared config |
| `list_regions` | `profile` | enabled regions |

**Compute**

| Tool | Input | Output (trimmed projection) |
|------|-------|-----------------------------|
| `list_ec2_instances` | `profile`, `region`, `max_items` | id, name tag, type, state, az, private_ip, public_ip |
| `list_lambda_functions` | `profile`, `region`, `max_items` | name, runtime, memory, last_modified |
| `list_ecs_clusters` | `profile`, `region`, `max_items` | cluster name/arn, status, running/active counts |
| `list_ecs_services` | `profile`, `region`, `cluster`, `max_items` | service, status, desired/running count, task_def |
| `list_autoscaling_groups` | `profile`, `region`, `max_items` | name, min/max/desired, instance count |

**Storage & DB**

| Tool | Input | Output (trimmed projection) |
|------|-------|-----------------------------|
| `list_s3_buckets` | `profile` | name, region, creation_date |
| `list_ebs_volumes` | `profile`, `region`, `max_items` | id, size, state, type, attached_instance |
| `list_rds_instances` | `profile`, `region`, `max_items` | id, engine, class, status, endpoint |
| `list_dynamodb_tables` | `profile`, `region`, `max_items` | name, status, item_count, size_bytes |

**Networking**

| Tool | Input | Output (trimmed projection) |
|------|-------|-----------------------------|
| `list_vpcs` | `profile`, `region`, `max_items` | id, cidr, is_default, name tag |
| `list_subnets` | `profile`, `region`, `vpc_id?`, `max_items` | id, vpc_id, az, cidr, available_ips |
| `list_security_groups` | `profile`, `region`, `vpc_id?`, `max_items` | id, name, vpc_id, ingress/egress rule counts |
| `list_load_balancers` | `profile`, `region`, `max_items` | name, type, scheme, dns_name, state (ELBv2) |
| `list_elastic_ips` | `profile`, `region` | allocation_id, public_ip, association |

**Identity & cost**

| Tool | Input | Output (trimmed projection) |
|------|-------|-----------------------------|
| `list_iam_users` | `profile`, `max_items` | name, id, created, password_last_used |
| `list_iam_roles` | `profile`, `max_items` | name, id, created, trust summary |
| `list_iam_policies` | `profile`, `max_items` | name, arn, attachment_count (customer-managed only) |
| `get_cost_summary` | `profile`, `start?`, `end?` | MTD unblended cost by service (**Cost Explorer — paid call, ~$0.01/request**) |

## Architecture & layout

```
mcps/aws-resources/
  cmd/awsresources/main.go        # calls Run(ctx)
  internal/awsresources/
    server.go                     # NewServer(): register tools; Run()
    awsconfig.go                  # cached LoadDefaultConfig per profile|region; list_profiles, list_regions
    compute.go                    # ec2/lambda/ecs/asg handlers + In/Out structs
    storage.go                    # s3/ebs/rds/dynamodb
    network.go                    # vpc/subnet/sg/elb/eip
    identity.go                   # iam users/roles/policies
    cost.go                       # cost explorer
    awsconfig_test.go compute_test.go storage_test.go network_test.go identity_test.go cost_test.go server_test.go
  README.md  Makefile  go.mod  .copier-answers.yml
```

- Each handler is a **free function** depending on a **narrow per-call
  interface** (e.g. `type describeInstancesAPI interface { DescribeInstances(ctx, *ec2.DescribeInstancesInput, ...) (*ec2.DescribeInstancesOutput, error) }`),
  not the concrete client — so it unit-tests with a fake and no live AWS.
- `server.go` builds real service clients from the cached `aws.Config` and binds
  them into each tool via thin closures.
- Input/output structs carry `json` + `jsonschema` tags for auto-generated
  schemas.

## Data flow

1. Tool call over stdio → SDK decodes args (incl. `profile`/`region`).
2. Handler obtains the cached `aws.Config` for that `profile|region`, constructs
   (or reuses) the service client.
3. Handler calls the read-only AWS API, following the paginator up to
   `max_items`.
4. Response mapped to the trimmed output struct → `CallToolResult`.

## Error handling

- Missing profile / expired SSO → actionable tool error.
- Regional tool called without `region` → validation error before any AWS call.
- AWS API errors (AccessDenied, throttling/`ThrottlingException`, not found) →
  readable tool error carrying service + error code; no panics leak.
- Pagination: `max_items` (default 100, capped at 1000); truncation reported in
  the result.

## Security

- **Read-only by construction:** only Describe/List/Get APIs are imported; no
  mutating client method is referenced anywhere in the unit.
- No credentials logged — profile names only.
- IAM tools return **metadata only** (names, ARNs, dates, attachment counts) —
  never policy documents, keys, or secrets.
- `~/.aws/config` is read solely to enumerate profile names.
- `get_cost_summary` is the only tool that incurs an AWS charge; documented in
  the README so it is called deliberately.

## Testing

- Per-file fakes implementing the narrow interfaces, returning canned SDK output
  structs.
- Coverage: happy path per tool; pagination/`max_items` cap; error mapping
  (AccessDenied, throttling); missing-`region` validation; `list_profiles`
  parsing a fixture `~/.aws/config`; cost-summary date defaulting.
- Fully offline and deterministic. `make test` (`go test -race ./...`),
  `make lint` (`gofmt -l` + `go vet` + `staticcheck`).

## Repo integration (coordinator)

- README: purpose, language rationale, run/build/test commands, required IAM
  read permissions (e.g. `ec2:Describe*`, `s3:ListAllMyBuckets`, `rds:Describe*`,
  `dynamodb:ListTables/DescribeTable`, `elasticloadbalancing:Describe*`,
  `autoscaling:Describe*`, `lambda:ListFunctions`, `ecs:List*/Describe*`,
  `iam:List*/Get*`, `ce:GetCostAndUsage`), and the Cost Explorer charge note.
