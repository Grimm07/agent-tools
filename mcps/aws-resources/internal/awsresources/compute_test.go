package awsresources

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// fakeEC2Instances implements ec2.DescribeInstancesAPIClient (the SDK-provided
// paginator interface) with optional multi-page output.
type fakeEC2Instances struct {
	pages []*ec2.DescribeInstancesOutput
	err   error
	calls int
}

func (f *fakeEC2Instances) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	page := f.pages[f.calls]
	f.calls++
	return page, nil
}

func TestListEC2Instances(t *testing.T) {
	f := &fakeEC2Instances{pages: []*ec2.DescribeInstancesOutput{{
		Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{
			InstanceId:       aws.String("i-123"),
			InstanceType:     ec2types.InstanceTypeT3Micro,
			State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			Placement:        &ec2types.Placement{AvailabilityZone: aws.String("us-east-1a")},
			PrivateIpAddress: aws.String("10.0.0.1"),
			PublicIpAddress:  aws.String("1.2.3.4"),
			Tags:             []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web")}},
		}}}},
	}}}
	out, err := listEC2Instances(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Instances) != 1 {
		t.Fatalf("got %d instances, want 1: %+v", len(out.Instances), out.Instances)
	}
	got := out.Instances[0]
	if got.ID != "i-123" || got.Name != "web" || got.Type != "t3.micro" ||
		got.State != "running" || got.AZ != "us-east-1a" ||
		got.PrivateIP != "10.0.0.1" || got.PublicIP != "1.2.3.4" {
		t.Fatalf("unexpected projection: %+v", got)
	}
	if out.Truncated {
		t.Error("did not expect truncation")
	}
}

// max_items must cap results across pages and mark the result truncated.
func TestListEC2InstancesTruncates(t *testing.T) {
	inst := func(id string) ec2types.Instance {
		return ec2types.Instance{InstanceId: aws.String(id), State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning}}
	}
	f := &fakeEC2Instances{pages: []*ec2.DescribeInstancesOutput{
		{
			NextToken:    aws.String("p2"),
			Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{inst("i-1"), inst("i-2")}}},
		},
		{
			Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{inst("i-3"), inst("i-4")}}},
		},
	}}
	out, err := listEC2Instances(context.Background(), f, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Instances) != 3 {
		t.Fatalf("got %d, want cap of 3", len(out.Instances))
	}
	if !out.Truncated {
		t.Error("expected truncated=true")
	}
}

func TestListEC2InstancesMapsError(t *testing.T) {
	f := &fakeEC2Instances{err: apiErr{code: "AccessDenied", msg: "nope"}}
	_, err := listEC2Instances(context.Background(), f, 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTagValue(t *testing.T) {
	tags := []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("web")}, {Key: aws.String("env"), Value: aws.String("prod")}}
	if got := tagValue(tags, "Name"); got != "web" {
		t.Errorf("Name tag = %q", got)
	}
	if got := tagValue(tags, "missing"); got != "" {
		t.Errorf("missing tag = %q, want empty", got)
	}
}
