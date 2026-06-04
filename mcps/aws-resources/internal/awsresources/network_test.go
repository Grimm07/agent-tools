package awsresources

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// --- vpcs ------------------------------------------------------------------

type fakeVPCs struct {
	pages []*ec2.DescribeVpcsOutput
	calls int
}

func (f *fakeVPCs) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListVPCs(t *testing.T) {
	f := &fakeVPCs{pages: []*ec2.DescribeVpcsOutput{{Vpcs: []ec2types.Vpc{{
		VpcId:     aws.String("vpc-1"),
		CidrBlock: aws.String("10.0.0.0/16"),
		IsDefault: aws.Bool(true),
		Tags:      []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("main")}},
	}}}}}
	out, err := listVPCs(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	v := out.VPCs[0]
	if v.ID != "vpc-1" || v.CIDR != "10.0.0.0/16" || !v.IsDefault || v.Name != "main" {
		t.Fatalf("projection: %+v", v)
	}
}

// --- subnets ---------------------------------------------------------------

type fakeSubnets struct {
	pages  []*ec2.DescribeSubnetsOutput
	calls  int
	lastIn *ec2.DescribeSubnetsInput
}

func (f *fakeSubnets) DescribeSubnets(_ context.Context, in *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	f.lastIn = in
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListSubnets(t *testing.T) {
	f := &fakeSubnets{pages: []*ec2.DescribeSubnetsOutput{{Subnets: []ec2types.Subnet{{
		SubnetId:                aws.String("subnet-1"),
		VpcId:                   aws.String("vpc-1"),
		AvailabilityZone:        aws.String("us-east-1a"),
		CidrBlock:               aws.String("10.0.1.0/24"),
		AvailableIpAddressCount: aws.Int32(250),
	}}}}}
	out, err := listSubnets(context.Background(), f, "vpc-1", 100)
	if err != nil {
		t.Fatal(err)
	}
	s := out.Subnets[0]
	if s.ID != "subnet-1" || s.VpcID != "vpc-1" || s.AZ != "us-east-1a" || s.CIDR != "10.0.1.0/24" || s.AvailableIPs != 250 {
		t.Fatalf("projection: %+v", s)
	}
	// vpc_id filter must be applied.
	if f.lastIn == nil || len(f.lastIn.Filters) != 1 || aws.ToString(f.lastIn.Filters[0].Name) != "vpc-id" {
		t.Fatalf("vpc-id filter not applied: %+v", f.lastIn)
	}
}

func TestListSubnetsNoFilter(t *testing.T) {
	f := &fakeSubnets{pages: []*ec2.DescribeSubnetsOutput{{}}}
	if _, err := listSubnets(context.Background(), f, "", 100); err != nil {
		t.Fatal(err)
	}
	if len(f.lastIn.Filters) != 0 {
		t.Fatalf("expected no filters, got %+v", f.lastIn.Filters)
	}
}

// --- security groups -------------------------------------------------------

type fakeSGs struct {
	pages  []*ec2.DescribeSecurityGroupsOutput
	calls  int
	lastIn *ec2.DescribeSecurityGroupsInput
}

func (f *fakeSGs) DescribeSecurityGroups(_ context.Context, in *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	f.lastIn = in
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListSecurityGroups(t *testing.T) {
	f := &fakeSGs{pages: []*ec2.DescribeSecurityGroupsOutput{{SecurityGroups: []ec2types.SecurityGroup{{
		GroupId:             aws.String("sg-1"),
		GroupName:           aws.String("web"),
		VpcId:               aws.String("vpc-1"),
		IpPermissions:       []ec2types.IpPermission{{}, {}},
		IpPermissionsEgress: []ec2types.IpPermission{{}},
	}}}}}
	out, err := listSecurityGroups(context.Background(), f, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	g := out.Groups[0]
	if g.ID != "sg-1" || g.Name != "web" || g.VpcID != "vpc-1" || g.IngressRules != 2 || g.EgressRules != 1 {
		t.Fatalf("projection: %+v", g)
	}
}

// --- load balancers --------------------------------------------------------

type fakeELB struct {
	pages []*elasticloadbalancingv2.DescribeLoadBalancersOutput
	calls int
}

func (f *fakeELB) DescribeLoadBalancers(_ context.Context, _ *elasticloadbalancingv2.DescribeLoadBalancersInput, _ ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListLoadBalancers(t *testing.T) {
	f := &fakeELB{pages: []*elasticloadbalancingv2.DescribeLoadBalancersOutput{{LoadBalancers: []elbtypes.LoadBalancer{{
		LoadBalancerName: aws.String("app-lb"),
		Type:             elbtypes.LoadBalancerTypeEnumApplication,
		Scheme:           elbtypes.LoadBalancerSchemeEnumInternetFacing,
		DNSName:          aws.String("app-lb-123.elb.amazonaws.com"),
		State:            &elbtypes.LoadBalancerState{Code: elbtypes.LoadBalancerStateEnumActive},
	}}}}}
	out, err := listLoadBalancers(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	lb := out.LoadBalancers[0]
	if lb.Name != "app-lb" || lb.Type != "application" || lb.Scheme != "internet-facing" ||
		lb.DNSName != "app-lb-123.elb.amazonaws.com" || lb.State != "active" {
		t.Fatalf("projection: %+v", lb)
	}
}

// --- elastic IPs -----------------------------------------------------------

type fakeEIPs struct {
	out *ec2.DescribeAddressesOutput
}

func (f fakeEIPs) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return f.out, nil
}

func TestListElasticIPs(t *testing.T) {
	f := fakeEIPs{out: &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{
		{AllocationId: aws.String("eipalloc-1"), PublicIp: aws.String("1.2.3.4"), InstanceId: aws.String("i-9")},
		{AllocationId: aws.String("eipalloc-2"), PublicIp: aws.String("5.6.7.8")},
	}}}
	out, err := listElasticIPs(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Addresses) != 2 {
		t.Fatalf("got %d", len(out.Addresses))
	}
	if out.Addresses[0].Association != "i-9" {
		t.Fatalf("expected association i-9, got %q", out.Addresses[0].Association)
	}
	if out.Addresses[1].Association != "" {
		t.Fatalf("unassociated EIP must have empty association, got %q", out.Addresses[1].Association)
	}
}
