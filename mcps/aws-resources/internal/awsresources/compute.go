package awsresources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
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

// --- list_lambda_functions -------------------------------------------------

// LambdaFunction is a trimmed projection of a Lambda function.
type LambdaFunction struct {
	Name         string `json:"name"`
	Runtime      string `json:"runtime"`
	Memory       int32  `json:"memory"`
	LastModified string `json:"last_modified"`
}

// ListLambdaOutput is the structured result of list_lambda_functions.
type ListLambdaOutput struct {
	Functions []LambdaFunction `json:"functions"`
	Truncated bool             `json:"truncated"`
}

// listLambdaFunctions walks Lambda function pages up to max.
func listLambdaFunctions(ctx context.Context, api lambda.ListFunctionsAPIClient, max int) (ListLambdaOutput, error) {
	var out ListLambdaOutput
	p := lambda.NewListFunctionsPaginator(api, &lambda.ListFunctionsInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListLambdaOutput{}, mapAWSError("lambda", err)
		}
		for _, fn := range page.Functions {
			if len(out.Functions) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Functions = append(out.Functions, LambdaFunction{
				Name:         aws.ToString(fn.FunctionName),
				Runtime:      string(fn.Runtime),
				Memory:       aws.ToInt32(fn.MemorySize),
				LastModified: aws.ToString(fn.LastModified),
			})
		}
	}
	return out, nil
}

// --- list_ecs_clusters -----------------------------------------------------

// ecsClustersAPI is the narrow slice of the ECS client list_ecs_clusters needs:
// it lists cluster ARNs (paginated) then batch-describes them.
type ecsClustersAPI interface {
	ecs.ListClustersAPIClient
	DescribeClusters(context.Context, *ecs.DescribeClustersInput, ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
}

// ECSCluster is a trimmed projection of an ECS cluster.
type ECSCluster struct {
	Name           string `json:"name"`
	ARN            string `json:"arn"`
	Status         string `json:"status"`
	RunningTasks   int32  `json:"running_tasks"`
	ActiveServices int32  `json:"active_services"`
}

// ListECSClustersOutput is the structured result of list_ecs_clusters.
type ListECSClustersOutput struct {
	Clusters  []ECSCluster `json:"clusters"`
	Truncated bool         `json:"truncated"`
}

// listECSClusters lists cluster ARNs then describes them in batches of 100
// (the DescribeClusters limit), capping at max.
func listECSClusters(ctx context.Context, api ecsClustersAPI, max int) (ListECSClustersOutput, error) {
	var out ListECSClustersOutput
	var arns []string
	p := ecs.NewListClustersPaginator(api, &ecs.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListECSClustersOutput{}, mapAWSError("ecs", err)
		}
		arns = append(arns, page.ClusterArns...)
		if len(arns) >= max {
			arns = arns[:max]
			out.Truncated = true
			break
		}
	}
	for _, batch := range chunk(arns, 100) {
		desc, err := api.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: batch,
			Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldStatistics},
		})
		if err != nil {
			return ListECSClustersOutput{}, mapAWSError("ecs", err)
		}
		for _, c := range desc.Clusters {
			out.Clusters = append(out.Clusters, ECSCluster{
				Name:           aws.ToString(c.ClusterName),
				ARN:            aws.ToString(c.ClusterArn),
				Status:         aws.ToString(c.Status),
				RunningTasks:   c.RunningTasksCount,
				ActiveServices: c.ActiveServicesCount,
			})
		}
	}
	return out, nil
}

// --- list_ecs_services -----------------------------------------------------

// ecsServicesAPI is the narrow slice of the ECS client list_ecs_services needs.
type ecsServicesAPI interface {
	ecs.ListServicesAPIClient
	DescribeServices(context.Context, *ecs.DescribeServicesInput, ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

// ECSService is a trimmed projection of an ECS service.
type ECSService struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	DesiredCount   int32  `json:"desired_count"`
	RunningCount   int32  `json:"running_count"`
	TaskDefinition string `json:"task_def"`
}

// ListECSServicesOutput is the structured result of list_ecs_services.
type ListECSServicesOutput struct {
	Services  []ECSService `json:"services"`
	Truncated bool         `json:"truncated"`
}

// listECSServices lists service ARNs in a cluster then describes them in
// batches of 10 (the DescribeServices limit), capping at max.
func listECSServices(ctx context.Context, api ecsServicesAPI, cluster string, max int) (ListECSServicesOutput, error) {
	var out ListECSServicesOutput
	var arns []string
	p := ecs.NewListServicesPaginator(api, &ecs.ListServicesInput{Cluster: aws.String(cluster)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListECSServicesOutput{}, mapAWSError("ecs", err)
		}
		arns = append(arns, page.ServiceArns...)
		if len(arns) >= max {
			arns = arns[:max]
			out.Truncated = true
			break
		}
	}
	for _, batch := range chunk(arns, 10) {
		desc, err := api.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: batch,
		})
		if err != nil {
			return ListECSServicesOutput{}, mapAWSError("ecs", err)
		}
		for _, sv := range desc.Services {
			out.Services = append(out.Services, ECSService{
				Name:           aws.ToString(sv.ServiceName),
				Status:         aws.ToString(sv.Status),
				DesiredCount:   sv.DesiredCount,
				RunningCount:   sv.RunningCount,
				TaskDefinition: aws.ToString(sv.TaskDefinition),
			})
		}
	}
	return out, nil
}

// --- list_autoscaling_groups -----------------------------------------------

// AutoScalingGroup is a trimmed projection of an Auto Scaling group.
type AutoScalingGroup struct {
	Name          string `json:"name"`
	Min           int32  `json:"min"`
	Max           int32  `json:"max"`
	Desired       int32  `json:"desired"`
	InstanceCount int    `json:"instance_count"`
}

// ListASGOutput is the structured result of list_autoscaling_groups.
type ListASGOutput struct {
	Groups    []AutoScalingGroup `json:"groups"`
	Truncated bool               `json:"truncated"`
}

// listAutoScalingGroups walks Auto Scaling group pages up to max.
func listAutoScalingGroups(ctx context.Context, api autoscaling.DescribeAutoScalingGroupsAPIClient, max int) (ListASGOutput, error) {
	var out ListASGOutput
	p := autoscaling.NewDescribeAutoScalingGroupsPaginator(api, &autoscaling.DescribeAutoScalingGroupsInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListASGOutput{}, mapAWSError("autoscaling", err)
		}
		for _, g := range page.AutoScalingGroups {
			if len(out.Groups) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Groups = append(out.Groups, AutoScalingGroup{
				Name:          aws.ToString(g.AutoScalingGroupName),
				Min:           aws.ToInt32(g.MinSize),
				Max:           aws.ToInt32(g.MaxSize),
				Desired:       aws.ToInt32(g.DesiredCapacity),
				InstanceCount: len(g.Instances),
			})
		}
	}
	return out, nil
}

// chunk splits s into consecutive slices of at most size elements.
func chunk[T any](s []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	var out [][]T
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
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

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_lambda_functions",
		Description: "List Lambda functions in a region (name, runtime, memory, last_modified).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListLambdaOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListLambdaOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListLambdaOutput{}, err
		}
		out, err := listLambdaFunctions(ctx, lambda.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListLambdaOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_ecs_clusters",
		Description: "List ECS clusters in a region (name/arn, status, running tasks, active services).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListECSClustersOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListECSClustersOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListECSClustersOutput{}, err
		}
		out, err := listECSClusters(ctx, ecs.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListECSClustersOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_ecs_services",
		Description: "List ECS services in a cluster (service, status, desired/running count, task_def).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListECSServicesInput) (*mcp.CallToolResult, ListECSServicesOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListECSServicesOutput{}, err
		}
		if in.Cluster == "" {
			return nil, ListECSServicesOutput{}, errClusterRequired
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListECSServicesOutput{}, err
		}
		out, err := listECSServices(ctx, ecs.NewFromConfig(cfg), in.Cluster, clampMax(in.MaxItems))
		if err != nil {
			return nil, ListECSServicesOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_autoscaling_groups",
		Description: "List Auto Scaling groups in a region (name, min/max/desired, instance count).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListASGOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListASGOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListASGOutput{}, err
		}
		out, err := listAutoScalingGroups(ctx, autoscaling.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListASGOutput{}, err
		}
		return textResult(out), out, nil
	})
}

// ListECSServicesInput adds a required cluster arg to the standard regional input.
type ListECSServicesInput struct {
	Profile  string `json:"profile" jsonschema:"AWS profile name"`
	Region   string `json:"region" jsonschema:"AWS region"`
	Cluster  string `json:"cluster" jsonschema:"ECS cluster name or ARN (required)"`
	MaxItems int    `json:"max_items,omitempty" jsonschema:"max results (default 100, cap 1000)"`
}
