# AWS Resources

A read-only [MCP](https://modelcontextprotocol.io) server that lets an agent **view live AWS
resources** — compute, storage & databases, networking, and identity & cost — across the owner's
accounts, targeting multiple named profiles from `~/.aws/config`.

Built on the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) and
[`aws-sdk-go-v2`](https://github.com/aws/aws-sdk-go-v2).

## Read-only by construction

Every tool calls **only** `Describe*` / `List*` / `Get*` AWS APIs. No mutating client method is
imported or referenced anywhere in the unit — this is enforced in review with a grep guard (see
[Verifying read-only](#verifying-read-only)). IAM tools return **metadata only** (names, IDs, dates,
attachment counts, a short trust summary) and never policy documents, access keys, or secrets.

## Language choice

**Go**, consistent with the other MCP units: this server ships as a single static binary, runs as a
long-lived stdio process, and uses the official `aws-sdk-go-v2`. That matches the repo's rule of
reserving Go for binaries/daemons distributed to other machines (see the root `CLAUDE.md`).

## Requirements

- Go ≥ 1.25 (the MCP SDK requires it). With `GOTOOLCHAIN=auto` (the default), the right toolchain
  downloads on first build.
- AWS credentials resolvable through the **default credential chain** — environment variables, SSO, or
  shared config. Nothing is hardcoded; SSO profiles work transparently.

## Commands

```bash
make test    # go test -race ./...   (fully offline; no AWS calls)
make build   # -> bin/aws-resources
make run     # serve over stdio
make lint    # gofmt check + go vet + staticcheck (if installed)
make tidy    # go mod tidy
```

## Authentication, profiles & regions

- Account model: **multiple named profiles** from `~/.aws/config` (SSO-friendly). Use `list_profiles`
  to enumerate them.
- **Regional** tools take `profile` + `region` (e.g. `list_ec2_instances`). A missing `region` is
  rejected before any AWS call.
- **Global** services take `profile` only: `list_s3_buckets`, the IAM tools, and `get_cost_summary`.
- The resolved `aws.Config` is cached per `profile|region`, so repeated calls in a session avoid
  re-resolving SSO/shared config.
- Invalid or expired credentials surface an actionable error (e.g. "try: `aws sso login --profile X`").
- `~/.aws/config` is read **solely to enumerate profile names** (honoring `AWS_CONFIG_FILE`).

## Pagination

List tools accept `max_items` (default 100, hard cap 1000) and report `truncated: true` when more
results exist beyond the cap.

## Tools

### Helpers
| Tool | Input | Output |
|------|-------|--------|
| `list_profiles` | — | profile names from `~/.aws/config` (+ source path) |
| `list_regions` | `profile`, `region?` | enabled regions (name, opt-in status) |

### Compute
| Tool | Input | Output |
|------|-------|--------|
| `list_ec2_instances` | `profile`, `region`, `max_items?` | id, name tag, type, state, az, private/public IP |
| `list_lambda_functions` | `profile`, `region`, `max_items?` | name, runtime, memory, last_modified |
| `list_ecs_clusters` | `profile`, `region`, `max_items?` | name/arn, status, running tasks, active services |
| `list_ecs_services` | `profile`, `region`, `cluster`, `max_items?` | service, status, desired/running, task_def |
| `list_autoscaling_groups` | `profile`, `region`, `max_items?` | name, min/max/desired, instance count |

### Storage & DB
| Tool | Input | Output |
|------|-------|--------|
| `list_s3_buckets` | `profile` | name, region, creation_date |
| `list_ebs_volumes` | `profile`, `region`, `max_items?` | id, size, state, type, attached_instance |
| `list_rds_instances` | `profile`, `region`, `max_items?` | id, engine, class, status, endpoint |
| `list_dynamodb_tables` | `profile`, `region`, `max_items?` | name, status, item_count, size_bytes |

### Networking
| Tool | Input | Output |
|------|-------|--------|
| `list_vpcs` | `profile`, `region`, `max_items?` | id, cidr, is_default, name tag |
| `list_subnets` | `profile`, `region`, `vpc_id?`, `max_items?` | id, vpc_id, az, cidr, available_ips |
| `list_security_groups` | `profile`, `region`, `vpc_id?`, `max_items?` | id, name, vpc_id, ingress/egress rule counts |
| `list_load_balancers` | `profile`, `region`, `max_items?` | name, type, scheme, dns_name, state (ELBv2) |
| `list_elastic_ips` | `profile`, `region` | allocation_id, public_ip, association |

### Identity & cost
| Tool | Input | Output |
|------|-------|--------|
| `list_iam_users` | `profile`, `max_items?` | name, id, created, password_last_used |
| `list_iam_roles` | `profile`, `max_items?` | name, id, created, trust summary (path + description) |
| `list_iam_policies` | `profile`, `max_items?` | name, arn, attachment_count (customer-managed only) |
| `get_cost_summary` | `profile`, `start?`, `end?` | MTD unblended cost by service + total |

> **`get_cost_summary` is a paid call.** It uses AWS Cost Explorer
> (`GetCostAndUsage`), which bills **~$0.01 per request**. It is the only tool in this server that
> incurs an AWS charge — call it deliberately. `start`/`end` are `YYYY-MM-DD` (End is exclusive) and
> default to month-to-date.

## Required IAM read permissions

Grant the profile a least-privilege read-only policy covering the services you intend to query:

```
ec2:DescribeInstances, ec2:DescribeRegions, ec2:DescribeVolumes, ec2:DescribeVpcs,
ec2:DescribeSubnets, ec2:DescribeSecurityGroups, ec2:DescribeAddresses
lambda:ListFunctions
ecs:ListClusters, ecs:DescribeClusters, ecs:ListServices, ecs:DescribeServices
autoscaling:DescribeAutoScalingGroups
s3:ListAllMyBuckets, s3:GetBucketLocation
rds:DescribeDBInstances
dynamodb:ListTables, dynamodb:DescribeTable
elasticloadbalancing:DescribeLoadBalancers
iam:ListUsers, iam:ListRoles, iam:ListPolicies
ce:GetCostAndUsage
```

The AWS-managed `ReadOnlyAccess` (or `ViewOnlyAccess`) policy is a superset and works for all tools.

## Architecture

Each tool is a free function depending on a **narrow per-call interface** — usually the SDK's own
paginator interface (e.g. `ec2.DescribeInstancesAPIClient`), or a hand-rolled one-method interface for
non-paginated calls (`s3.ListBuckets`, `ec2.DescribeAddresses`). Tests inject fakes implementing those
interfaces, so the whole suite runs offline with no live AWS. `server.go` builds the real service
clients from the cached `aws.Config` and binds them into each tool via thin closures.

```
cmd/aws-resources/main.go           # entrypoint: wires the stdio transport
internal/awsresources/
  server.go        # NewServer(): registers every tool group; Run()
  awsconfig.go     # cached LoadDefaultConfig per profile|region; profile parsing
  helpers.go       # list_profiles, list_regions
  compute.go       # ec2 / lambda / ecs / autoscaling
  storage.go       # s3 / ebs / rds / dynamodb
  network.go       # vpc / subnet / sg / elb / eip
  identity.go      # iam users / roles / policies (metadata only)
  cost.go          # cost explorer
  result.go        # textResult + mapAWSError (smithy.APIError aware)
  validate.go      # regionRequired / clampMax
  util.go          # formatTime / itoa
  *_test.go        # per-file fakes; fully offline
```

## Verifying read-only

```bash
grep -rE "(Create|Delete|Put|Update|Modify|Terminate|Run)[A-Z]" internal/ | grep -v CreateDate
```

This must print nothing. (The only matches in the tree are the IAM `CreateDate` *field* — read-only
metadata, not an API call — so it is filtered out above.)

## Updating from the template

```bash
copier update --trust   # pulls template improvements; answers are in .copier-answers.yml
```
