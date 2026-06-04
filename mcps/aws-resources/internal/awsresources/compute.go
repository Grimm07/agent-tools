package awsresources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// EC2Instance is a trimmed projection of an EC2 instance.
type EC2Instance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	State     string `json:"state"`
	AZ        string `json:"az"`
	PrivateIP string `json:"private_ip"`
	PublicIP  string `json:"public_ip"`
}

// ListEC2Input selects the account/region and caps the result size.
type ListEC2Input struct {
	Profile  string `json:"profile" jsonschema:"AWS profile name"`
	Region   string `json:"region" jsonschema:"AWS region"`
	MaxItems int    `json:"max_items,omitempty" jsonschema:"max results (default 100, cap 1000)"`
}

// ListEC2Output is the structured result of list_ec2_instances.
type ListEC2Output struct {
	Instances []EC2Instance `json:"instances"`
	Truncated bool          `json:"truncated"`
}

// listEC2Instances is the testable core: it takes the SDK paginator interface,
// not a concrete client, so tests inject a fake. It walks pages until max
// results are collected, marking Truncated if more remain.
func listEC2Instances(ctx context.Context, api ec2.DescribeInstancesAPIClient, max int) (ListEC2Output, error) {
	var out ListEC2Output
	p := ec2.NewDescribeInstancesPaginator(api, &ec2.DescribeInstancesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListEC2Output{}, mapAWSError("ec2", err)
		}
		for _, r := range page.Reservations {
			for _, i := range r.Instances {
				if len(out.Instances) >= max {
					out.Truncated = true
					return out, nil
				}
				out.Instances = append(out.Instances, EC2Instance{
					ID:        aws.ToString(i.InstanceId),
					Name:      tagValue(i.Tags, "Name"),
					Type:      string(i.InstanceType),
					State:     stateName(i.State),
					AZ:        az(i.Placement),
					PrivateIP: aws.ToString(i.PrivateIpAddress),
					PublicIP:  aws.ToString(i.PublicIpAddress),
				})
			}
		}
	}
	return out, nil
}

// --- EC2 projection helpers ------------------------------------------------

// tagValue returns the value of the tag with the given key, or "" if absent.
func tagValue(tags []ec2types.Tag, key string) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == key {
			return aws.ToString(t.Value)
		}
	}
	return ""
}

// stateName returns the instance state name, or "" if unset.
func stateName(s *ec2types.InstanceState) string {
	if s == nil {
		return ""
	}
	return string(s.Name)
}

// az returns the placement availability zone, or "" if unset.
func az(p *ec2types.Placement) string {
	if p == nil {
		return ""
	}
	return aws.ToString(p.AvailabilityZone)
}

// registerComputeTools registers the compute-category tools.
func registerComputeTools(s *mcp.Server, cache *configCache) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_ec2_instances",
		Description: "List EC2 instances in a region (id, name tag, type, state, az, private/public IP).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListEC2Output, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListEC2Output{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListEC2Output{}, err
		}
		out, err := listEC2Instances(ctx, ec2.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListEC2Output{}, err
		}
		return textResult(out), out, nil
	})
}
