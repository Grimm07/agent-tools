package awsresources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- list_s3_buckets -------------------------------------------------------

// s3BucketsAPI is the narrow slice of the S3 client list_s3_buckets needs:
// ListBuckets is global, then GetBucketLocation resolves each bucket's region.
type s3BucketsAPI interface {
	ListBuckets(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	GetBucketLocation(context.Context, *s3.GetBucketLocationInput, ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

// S3Bucket is a trimmed projection of an S3 bucket.
type S3Bucket struct {
	Name         string `json:"name"`
	Region       string `json:"region"`
	CreationDate string `json:"creation_date"`
}

// ListS3Output is the structured result of list_s3_buckets.
type ListS3Output struct {
	Buckets []S3Bucket `json:"buckets"`
}

// ListS3Input selects the profile; S3 bucket listing is global, so no region.
type ListS3Input struct {
	Profile string `json:"profile" jsonschema:"AWS profile name"`
}

// listS3Buckets lists all buckets and resolves each one's region. An empty
// LocationConstraint denotes us-east-1 (an AWS quirk).
func listS3Buckets(ctx context.Context, api s3BucketsAPI) (ListS3Output, error) {
	var out ListS3Output
	res, err := api.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return out, mapAWSError("s3", err)
	}
	for _, b := range res.Buckets {
		name := aws.ToString(b.Name)
		region := ""
		loc, lerr := api.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(name)})
		if lerr == nil {
			region = string(loc.LocationConstraint)
			if region == "" {
				region = "us-east-1"
			}
		}
		out.Buckets = append(out.Buckets, S3Bucket{
			Name:         name,
			Region:       region,
			CreationDate: formatTime(b.CreationDate),
		})
	}
	return out, nil
}

// --- list_ebs_volumes ------------------------------------------------------

// EBSVolume is a trimmed projection of an EBS volume.
type EBSVolume struct {
	ID               string `json:"id"`
	Size             int32  `json:"size"`
	State            string `json:"state"`
	Type             string `json:"type"`
	AttachedInstance string `json:"attached_instance"`
}

// ListEBSOutput is the structured result of list_ebs_volumes.
type ListEBSOutput struct {
	Volumes   []EBSVolume `json:"volumes"`
	Truncated bool        `json:"truncated"`
}

// listEBSVolumes walks EBS volume pages up to max.
func listEBSVolumes(ctx context.Context, api ec2.DescribeVolumesAPIClient, max int) (ListEBSOutput, error) {
	var out ListEBSOutput
	p := ec2.NewDescribeVolumesPaginator(api, &ec2.DescribeVolumesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListEBSOutput{}, mapAWSError("ec2", err)
		}
		for _, v := range page.Volumes {
			if len(out.Volumes) >= max {
				out.Truncated = true
				return out, nil
			}
			attached := ""
			if len(v.Attachments) > 0 {
				attached = aws.ToString(v.Attachments[0].InstanceId)
			}
			out.Volumes = append(out.Volumes, EBSVolume{
				ID:               aws.ToString(v.VolumeId),
				Size:             aws.ToInt32(v.Size),
				State:            string(v.State),
				Type:             string(v.VolumeType),
				AttachedInstance: attached,
			})
		}
	}
	return out, nil
}

// --- list_rds_instances ----------------------------------------------------

// RDSInstance is a trimmed projection of an RDS DB instance.
type RDSInstance struct {
	ID       string `json:"id"`
	Engine   string `json:"engine"`
	Class    string `json:"class"`
	Status   string `json:"status"`
	Endpoint string `json:"endpoint"`
}

// ListRDSOutput is the structured result of list_rds_instances.
type ListRDSOutput struct {
	Instances []RDSInstance `json:"instances"`
	Truncated bool          `json:"truncated"`
}

// listRDSInstances walks RDS DB instance pages up to max.
func listRDSInstances(ctx context.Context, api rds.DescribeDBInstancesAPIClient, max int) (ListRDSOutput, error) {
	var out ListRDSOutput
	p := rds.NewDescribeDBInstancesPaginator(api, &rds.DescribeDBInstancesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListRDSOutput{}, mapAWSError("rds", err)
		}
		for _, db := range page.DBInstances {
			if len(out.Instances) >= max {
				out.Truncated = true
				return out, nil
			}
			endpoint := ""
			if db.Endpoint != nil {
				endpoint = aws.ToString(db.Endpoint.Address)
				if p := aws.ToInt32(db.Endpoint.Port); p != 0 {
					endpoint = endpoint + ":" + itoa(int(p))
				}
			}
			out.Instances = append(out.Instances, RDSInstance{
				ID:       aws.ToString(db.DBInstanceIdentifier),
				Engine:   aws.ToString(db.Engine),
				Class:    aws.ToString(db.DBInstanceClass),
				Status:   aws.ToString(db.DBInstanceStatus),
				Endpoint: endpoint,
			})
		}
	}
	return out, nil
}

// --- list_dynamodb_tables --------------------------------------------------

// dynamoTablesAPI is the narrow slice of the DynamoDB client this tool needs:
// ListTables (paginated) plus a per-table DescribeTable for status/size.
type dynamoTablesAPI interface {
	dynamodb.ListTablesAPIClient
	DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

// DynamoTable is a trimmed projection of a DynamoDB table.
type DynamoTable struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	ItemCount int64  `json:"item_count"`
	SizeBytes int64  `json:"size_bytes"`
}

// ListDynamoOutput is the structured result of list_dynamodb_tables.
type ListDynamoOutput struct {
	Tables    []DynamoTable `json:"tables"`
	Truncated bool          `json:"truncated"`
}

// listDynamoDBTables lists table names then describes each one, capping at max.
func listDynamoDBTables(ctx context.Context, api dynamoTablesAPI, max int) (ListDynamoOutput, error) {
	var out ListDynamoOutput
	var names []string
	p := dynamodb.NewListTablesPaginator(api, &dynamodb.ListTablesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListDynamoOutput{}, mapAWSError("dynamodb", err)
		}
		names = append(names, page.TableNames...)
		if len(names) >= max {
			names = names[:max]
			out.Truncated = true
			break
		}
	}
	for _, name := range names {
		desc, err := api.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(name)})
		if err != nil {
			return ListDynamoOutput{}, mapAWSError("dynamodb", err)
		}
		t := desc.Table
		if t == nil {
			continue
		}
		out.Tables = append(out.Tables, DynamoTable{
			Name:      aws.ToString(t.TableName),
			Status:    string(t.TableStatus),
			ItemCount: aws.ToInt64(t.ItemCount),
			SizeBytes: aws.ToInt64(t.TableSizeBytes),
		})
	}
	return out, nil
}

// registerStorageTools registers the storage & database tools.
func registerStorageTools(s *mcp.Server, cache *configCache) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_s3_buckets",
		Description: "List S3 buckets for a profile (name, region, creation_date). Global call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListS3Input) (*mcp.CallToolResult, ListS3Output, error) {
		cfg, err := cache.get(ctx, in.Profile, "")
		if err != nil {
			return nil, ListS3Output{}, err
		}
		out, err := listS3Buckets(ctx, s3.NewFromConfig(cfg))
		if err != nil {
			return nil, ListS3Output{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_ebs_volumes",
		Description: "List EBS volumes in a region (id, size, state, type, attached_instance).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListEBSOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListEBSOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListEBSOutput{}, err
		}
		out, err := listEBSVolumes(ctx, ec2.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListEBSOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_rds_instances",
		Description: "List RDS DB instances in a region (id, engine, class, status, endpoint).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListRDSOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListRDSOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListRDSOutput{}, err
		}
		out, err := listRDSInstances(ctx, rds.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListRDSOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_dynamodb_tables",
		Description: "List DynamoDB tables in a region (name, status, item_count, size_bytes).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListEC2Input) (*mcp.CallToolResult, ListDynamoOutput, error) {
		if err := regionRequired(in.Region); err != nil {
			return nil, ListDynamoOutput{}, err
		}
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListDynamoOutput{}, err
		}
		out, err := listDynamoDBTables(ctx, dynamodb.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListDynamoOutput{}, err
		}
		return textResult(out), out, nil
	})
}
