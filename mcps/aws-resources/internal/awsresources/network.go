package awsresources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// vpcFilter builds an optional "vpc-id" filter slice; an empty id yields nil so
// the AWS call is unfiltered.
func vpcFilter(vpcID string) []ec2types.Filter {
	if vpcID == "" {
		return nil
	}
	return []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}}
}

// --- list_vpcs -------------------------------------------------------------

// VPC is a trimmed projection of a VPC.
type VPC struct {
	ID        string `json:"id"`
	CIDR      string `json:"cidr"`
	IsDefault bool   `json:"is_default"`
	Name      string `json:"name"`
}

// ListVPCsOutput is the structured result of list_vpcs.
type ListVPCsOutput struct {
	VPCs      []VPC `json:"vpcs"`
	Truncated bool  `json:"truncated"`
}

// listVPCs walks VPC pages up to max.
func listVPCs(ctx context.Context, api ec2.DescribeVpcsAPIClient, max int) (ListVPCsOutput, error) {
	var out ListVPCsOutput
	p := ec2.NewDescribeVpcsPaginator(api, &ec2.DescribeVpcsInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListVPCsOutput{}, mapAWSError("ec2", err)
		}
		for _, v := range page.Vpcs {
			if len(out.VPCs) >= max {
				out.Truncated = true
				return out, nil
			}
			out.VPCs = append(out.VPCs, VPC{
				ID:        aws.ToString(v.VpcId),
				CIDR:      aws.ToString(v.CidrBlock),
				IsDefault: aws.ToBool(v.IsDefault),
				Name:      tagValue(v.Tags, "Name"),
			})
		}
	}
	return out, nil
}

// --- list_subnets ----------------------------------------------------------

// Subnet is a trimmed projection of a subnet.
type Subnet struct {
	ID           string `json:"id"`
	VpcID        string `json:"vpc_id"`
	AZ           string `json:"az"`
	CIDR         string `json:"cidr"`
	AvailableIPs int32  `json:"available_ips"`
}

// ListSubnetsInput optionally filters subnets by VPC.
type ListSubnetsInput struct {
	Profile  string `json:"profile" jsonschema:"AWS profile name"`
	Region   string `json:"region" jsonschema:"AWS region"`
	VpcID    string `json:"vpc_id,omitempty" jsonschema:"optional VPC id to filter by"`
	MaxItems int    `json:"max_items,omitempty" jsonschema:"max results (default 100, cap 1000)"`
}

// ListSubnetsOutput is the structured result of list_subnets.
type ListSubnetsOutput struct {
	Subnets   []Subnet `json:"subnets"`
	Truncated bool     `json:"truncated"`
}

// listSubnets walks subnet pages up to max, optionally filtered by VPC.
func listSubnets(ctx context.Context, api ec2.DescribeSubnetsAPIClient, vpcID string, max int) (ListSubnetsOutput, error) {
	var out ListSubnetsOutput
	p := ec2.NewDescribeSubnetsPaginator(api, &ec2.DescribeSubnetsInput{Filters: vpcFilter(vpcID)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListSubnetsOutput{}, mapAWSError("ec2", err)
		}
		for _, sn := range page.Subnets {
			if len(out.Subnets) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Subnets = append(out.Subnets, Subnet{
				ID:           aws.ToString(sn.SubnetId),
				VpcID:        aws.ToString(sn.VpcId),
				AZ:           aws.ToString(sn.AvailabilityZone),
				CIDR:         aws.ToString(sn.CidrBlock),
				AvailableIPs: aws.ToInt32(sn.AvailableIpAddressCount),
			})
		}
	}
	return out, nil
}

// --- list_security_groups --------------------------------------------------

// SecurityGroup is a trimmed projection of a security group. Rule contents are
// summarized as counts only.
type SecurityGroup struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	VpcID        string `json:"vpc_id"`
	IngressRules int    `json:"ingress_rules"`
	EgressRules  int    `json:"egress_rules"`
}

// ListSGOutput is the structured result of list_security_groups.
type ListSGOutput struct {
	Groups    []SecurityGroup `json:"groups"`
	Truncated bool            `json:"truncated"`
}

// listSecurityGroups walks security group pages up to max, optionally filtered
// by VPC.
func listSecurityGroups(ctx context.Context, api ec2.DescribeSecurityGroupsAPIClient, vpcID string, max int) (ListSGOutput, error) {
	var out ListSGOutput
	p := ec2.NewDescribeSecurityGroupsPaginator(api, &ec2.DescribeSecurityGroupsInput{Filters: vpcFilter(vpcID)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListSGOutput{}, mapAWSError("ec2", err)
		}
		for _, g := range page.SecurityGroups {
			if len(out.Groups) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Groups = append(out.Groups, SecurityGroup{
				ID:           aws.ToString(g.GroupId),
				Name:         aws.ToString(g.GroupName),
				VpcID:        aws.ToString(g.VpcId),
				IngressRules: len(g.IpPermissions),
				EgressRules:  len(g.IpPermissionsEgress),
			})
		}
	}
	return out, nil
}

// --- list_load_balancers ---------------------------------------------------

// LoadBalancer is a trimmed projection of an ELBv2 load balancer.
type LoadBalancer struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Scheme  string `json:"scheme"`
	DNSName string `json:"dns_name"`
	State   string `json:"state"`
}

// ListLBOutput is the structured result of list_load_balancers.
type ListLBOutput struct {
	LoadBalancers []LoadBalancer `json:"load_balancers"`
	Truncated     bool           `json:"truncated"`
}

// listLoadBalancers walks ELBv2 load balancer pages up to max.
func listLoadBalancers(ctx context.Context, api elasticloadbalancingv2.DescribeLoadBalancersAPIClient, max int) (ListLBOutput, error) {
	var out ListLBOutput
	p := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(api, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListLBOutput{}, mapAWSError("elasticloadbalancing", err)
		}
		for _, lb := range page.LoadBalancers {
			if len(out.LoadBalancers) >= max {
				out.Truncated = true
				return out, nil
			}
			state := ""
			if lb.State != nil {
				state = string(lb.State.Code)
			}
			out.LoadBalancers = append(out.LoadBalancers, LoadBalancer{
				Name:    aws.ToString(lb.LoadBalancerName),
				Type:    string(lb.Type),
				Scheme:  string(lb.Scheme),
				DNSName: aws.ToString(lb.DNSName),
				State:   state,
			})
		}
	}
	return out, nil
}

// --- list_elastic_ips ------------------------------------------------------

// describeAddressesAPI is the narrow slice of the EC2 client list_elastic_ips
// needs. DescribeAddresses is not paginated.
type describeAddressesAPI interface {
	DescribeAddresses(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
}

// ElasticIP is a trimmed projection of an Elastic IP allocation.
type ElasticIP struct {
	AllocationID string `json:"allocation_id"`
	PublicIP     string `json:"public_ip"`
	Association  string `json:"association"`
}

// ListEIPInput selects the account/region. DescribeAddresses is not paginated,
// so there is no max_items.
type ListEIPInput struct {
	Profile string `json:"profile" jsonschema:"AWS profile name"`
	Region  string `json:"region" jsonschema:"AWS region"`
}

// ListEIPOutput is the structured result of list_elastic_ips.
type ListEIPOutput struct {
	Addresses []ElasticIP `json:"addresses"`
}

// listElasticIPs returns all Elastic IPs. The association is the attached
// instance id, or the association id, or "" when unassociated.
func listElasticIPs(ctx context.Context, api describeAddressesAPI) (ListEIPOutput, error) {
	var out ListEIPOutput
	res, err := api.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return out, mapAWSError("ec2", err)
	}
	for _, a := range res.Addresses {
		assoc := aws.ToString(a.InstanceId)
		if assoc == "" {
			assoc = aws.ToString(a.AssociationId)
		}
		out.Addresses = append(out.Addresses, ElasticIP{
			AllocationID: aws.ToString(a.AllocationId),
			PublicIP:     aws.ToString(a.PublicIp),
			Association:  assoc,
		})
	}
	return out, nil
}

// registerNetworkTools registers the networking tools.
func registerNetworkTools(s *mcp.Server, cache *configCache) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_vpcs",
		Description: "List VPCs in a region (id, cidr, is_default, name tag).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListVPCsOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListVPCsOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListVPCsOutput{}, err
		}
		out, err := listVPCs(ctx, ec2.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListVPCsOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_subnets",
		Description: "List subnets in a region (id, vpc_id, az, cidr, available_ips). Optional vpc_id filter.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListSubnetsInput) (*mcp.CallToolResult, ListSubnetsOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListSubnetsOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListSubnetsOutput{}, err
		}
		out, err := listSubnets(ctx, ec2.NewFromConfig(cfg), in.VpcID, clampMax(in.MaxItems))
		if err != nil {
			return nil, ListSubnetsOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_security_groups",
		Description: "List security groups in a region (id, name, vpc_id, ingress/egress rule counts). Optional vpc_id filter.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListSubnetsInput) (*mcp.CallToolResult, ListSGOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListSGOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListSGOutput{}, err
		}
		out, err := listSecurityGroups(ctx, ec2.NewFromConfig(cfg), in.VpcID, clampMax(in.MaxItems))
		if err != nil {
			return nil, ListSGOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_load_balancers",
		Description: "List ELBv2 load balancers in a region (name, type, scheme, dns_name, state).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListLBOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListLBOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListLBOutput{}, err
		}
		out, err := listLoadBalancers(ctx, elasticloadbalancingv2.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListLBOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_elastic_ips",
		Description: "List Elastic IPs in a region (allocation_id, public_ip, association).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEIPInput) (*mcp.CallToolResult, ListEIPOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListEIPOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListEIPOutput{}, err
		}
		out, err := listElasticIPs(ctx, ec2.NewFromConfig(cfg))
		if err != nil {
			return nil, ListEIPOutput{}, err
		}
		return textResult(out), out, nil
	})
}
