package awsresources

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

type fakeRegions struct {
	out *ec2.DescribeRegionsOutput
	err error
}

func (f fakeRegions) DescribeRegions(context.Context, *ec2.DescribeRegionsInput, ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return f.out, f.err
}

func TestListRegions(t *testing.T) {
	f := fakeRegions{out: &ec2.DescribeRegionsOutput{Regions: []ec2types.Region{
		{RegionName: aws.String("us-east-1"), OptInStatus: aws.String("opt-in-not-required")},
		{RegionName: aws.String("eu-west-1"), OptInStatus: aws.String("opted-in")},
	}}}
	out, err := listRegions(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Regions) != 2 || out.Regions[0].Name != "us-east-1" || out.Regions[1].OptInStatus != "opted-in" {
		t.Fatalf("got %+v", out.Regions)
	}
}

// apiErr is a minimal smithy.APIError for exercising mapAWSError paths.
type apiErr struct {
	code, msg string
}

func (e apiErr) Error() string                 { return e.code + ": " + e.msg }
func (e apiErr) ErrorCode() string             { return e.code }
func (e apiErr) ErrorMessage() string          { return e.msg }
func (e apiErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestListRegionsMapsError(t *testing.T) {
	f := fakeRegions{err: apiErr{code: "UnauthorizedOperation", msg: "no"}}
	_, err := listRegions(context.Background(), f)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ec2") || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("error not mapped as expected: %v", err)
	}
}
