package awsresources

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// --- lambda ----------------------------------------------------------------

type fakeLambda struct {
	pages []*lambda.ListFunctionsOutput
	calls int
}

func (f *fakeLambda) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListLambdaFunctions(t *testing.T) {
	f := &fakeLambda{pages: []*lambda.ListFunctionsOutput{{Functions: []lambdatypes.FunctionConfiguration{{
		FunctionName: aws.String("fn"),
		Runtime:      lambdatypes.RuntimeGo1x,
		MemorySize:   aws.Int32(256),
		LastModified: aws.String("2024-01-01T00:00:00Z"),
	}}}}}
	out, err := listLambdaFunctions(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Functions) != 1 {
		t.Fatalf("got %d", len(out.Functions))
	}
	got := out.Functions[0]
	if got.Name != "fn" || got.Runtime != "go1.x" || got.Memory != 256 || got.LastModified != "2024-01-01T00:00:00Z" {
		t.Fatalf("projection: %+v", got)
	}
}

// --- ecs clusters ----------------------------------------------------------

type fakeECSClusters struct {
	listPages []*ecs.ListClustersOutput
	listCalls int
	desc      *ecs.DescribeClustersOutput
}

func (f *fakeECSClusters) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	p := f.listPages[f.listCalls]
	f.listCalls++
	return p, nil
}

func (f *fakeECSClusters) DescribeClusters(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return f.desc, nil
}

func TestListECSClusters(t *testing.T) {
	f := &fakeECSClusters{
		listPages: []*ecs.ListClustersOutput{{ClusterArns: []string{"arn:aws:ecs:us-east-1:1:cluster/web"}}},
		desc: &ecs.DescribeClustersOutput{Clusters: []ecstypes.Cluster{{
			ClusterName:         aws.String("web"),
			ClusterArn:          aws.String("arn:aws:ecs:us-east-1:1:cluster/web"),
			Status:              aws.String("ACTIVE"),
			RunningTasksCount:   3,
			ActiveServicesCount: 2,
		}}},
	}
	out, err := listECSClusters(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Clusters) != 1 {
		t.Fatalf("got %d", len(out.Clusters))
	}
	c := out.Clusters[0]
	if c.Name != "web" || c.Status != "ACTIVE" || c.RunningTasks != 3 || c.ActiveServices != 2 {
		t.Fatalf("projection: %+v", c)
	}
}

func TestListECSClustersEmpty(t *testing.T) {
	f := &fakeECSClusters{listPages: []*ecs.ListClustersOutput{{ClusterArns: nil}}}
	out, err := listECSClusters(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Clusters) != 0 {
		t.Fatalf("expected no clusters, got %+v", out.Clusters)
	}
}

// --- ecs services ----------------------------------------------------------

type fakeECSServices struct {
	listPages []*ecs.ListServicesOutput
	listCalls int
	desc      *ecs.DescribeServicesOutput
}

func (f *fakeECSServices) ListServices(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	p := f.listPages[f.listCalls]
	f.listCalls++
	return p, nil
}

func (f *fakeECSServices) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return f.desc, nil
}

func TestListECSServices(t *testing.T) {
	f := &fakeECSServices{
		listPages: []*ecs.ListServicesOutput{{ServiceArns: []string{"arn:svc/api"}}},
		desc: &ecs.DescribeServicesOutput{Services: []ecstypes.Service{{
			ServiceName:    aws.String("api"),
			Status:         aws.String("ACTIVE"),
			DesiredCount:   2,
			RunningCount:   2,
			TaskDefinition: aws.String("api:7"),
		}}},
	}
	out, err := listECSServices(context.Background(), f, "web", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Services) != 1 {
		t.Fatalf("got %d", len(out.Services))
	}
	s := out.Services[0]
	if s.Name != "api" || s.Status != "ACTIVE" || s.DesiredCount != 2 || s.RunningCount != 2 || s.TaskDefinition != "api:7" {
		t.Fatalf("projection: %+v", s)
	}
}

// --- autoscaling -----------------------------------------------------------

type fakeASG struct {
	pages []*autoscaling.DescribeAutoScalingGroupsOutput
	calls int
}

func (f *fakeASG) DescribeAutoScalingGroups(_ context.Context, _ *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListAutoScalingGroups(t *testing.T) {
	f := &fakeASG{pages: []*autoscaling.DescribeAutoScalingGroupsOutput{{AutoScalingGroups: []asgtypes.AutoScalingGroup{{
		AutoScalingGroupName: aws.String("web-asg"),
		MinSize:              aws.Int32(1),
		MaxSize:              aws.Int32(5),
		DesiredCapacity:      aws.Int32(3),
		Instances:            []asgtypes.Instance{{}, {}, {}},
	}}}}}
	out, err := listAutoScalingGroups(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Groups) != 1 {
		t.Fatalf("got %d", len(out.Groups))
	}
	g := out.Groups[0]
	if g.Name != "web-asg" || g.Min != 1 || g.Max != 5 || g.Desired != 3 || g.InstanceCount != 3 {
		t.Fatalf("projection: %+v", g)
	}
}
