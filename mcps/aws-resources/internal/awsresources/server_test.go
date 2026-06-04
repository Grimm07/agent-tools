package awsresources

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listRegisteredTools connects a client to the server over in-memory transports
// and returns the set of advertised tool names. Fully offline: it never invokes
// a tool, so no AWS call happens.
func listRegisteredTools(t *testing.T) map[string]bool {
	t.Helper()
	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()

	serverSession, err := NewServer().Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })

	res, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	return got
}

// TestServerBuilds verifies NewServer constructs and serves over a transport.
func TestServerBuilds(t *testing.T) {
	if NewServer() == nil {
		t.Fatal("NewServer returned nil")
	}
	_ = listRegisteredTools(t)
}

// TestServerRegistersAllTools is a registration regression guard: every tool the
// server promises must be advertised over the transport. Fully offline.
func TestServerRegistersAllTools(t *testing.T) {
	got := listRegisteredTools(t)
	want := []string{
		"list_profiles", "list_regions",
		"list_ec2_instances", "list_lambda_functions", "list_ecs_clusters",
		"list_ecs_services", "list_autoscaling_groups",
		"list_s3_buckets", "list_ebs_volumes", "list_rds_instances", "list_dynamodb_tables",
		"list_vpcs", "list_subnets", "list_security_groups", "list_load_balancers", "list_elastic_ips",
		"list_iam_users", "list_iam_roles", "list_iam_policies",
		"get_cost_summary",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q not registered", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("registered %d tools, want %d: %v", len(got), len(want), got)
	}
}
