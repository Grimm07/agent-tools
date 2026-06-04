package awsresources

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// --- s3 --------------------------------------------------------------------

type fakeS3 struct {
	buckets   *s3.ListBucketsOutput
	locations map[string]s3types.BucketLocationConstraint
}

func (f fakeS3) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return f.buckets, nil
}

func (f fakeS3) GetBucketLocation(_ context.Context, in *s3.GetBucketLocationInput, _ ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	return &s3.GetBucketLocationOutput{LocationConstraint: f.locations[aws.ToString(in.Bucket)]}, nil
}

func TestListS3Buckets(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	f := fakeS3{
		buckets: &s3.ListBucketsOutput{Buckets: []s3types.Bucket{
			{Name: aws.String("logs"), CreationDate: aws.Time(ts)},
			{Name: aws.String("us-east"), CreationDate: aws.Time(ts)},
		}},
		locations: map[string]s3types.BucketLocationConstraint{
			"logs":    s3types.BucketLocationConstraintEuWest1,
			"us-east": "", // empty constraint means us-east-1
		},
	}
	out, err := listS3Buckets(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Buckets) != 2 {
		t.Fatalf("got %d", len(out.Buckets))
	}
	if out.Buckets[0].Name != "logs" || out.Buckets[0].Region != "eu-west-1" {
		t.Fatalf("bucket0: %+v", out.Buckets[0])
	}
	if out.Buckets[1].Region != "us-east-1" {
		t.Fatalf("empty location must map to us-east-1, got %q", out.Buckets[1].Region)
	}
}

// --- ebs -------------------------------------------------------------------

type fakeVolumes struct {
	pages []*ec2.DescribeVolumesOutput
	calls int
}

func (f *fakeVolumes) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListEBSVolumes(t *testing.T) {
	f := &fakeVolumes{pages: []*ec2.DescribeVolumesOutput{{Volumes: []ec2types.Volume{{
		VolumeId:    aws.String("vol-1"),
		Size:        aws.Int32(100),
		State:       ec2types.VolumeStateInUse,
		VolumeType:  ec2types.VolumeTypeGp3,
		Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-9")}},
	}}}}}
	out, err := listEBSVolumes(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	v := out.Volumes[0]
	if v.ID != "vol-1" || v.Size != 100 || v.State != "in-use" || v.Type != "gp3" || v.AttachedInstance != "i-9" {
		t.Fatalf("projection: %+v", v)
	}
}

// --- rds -------------------------------------------------------------------

type fakeRDS struct {
	pages []*rds.DescribeDBInstancesOutput
	calls int
}

func (f *fakeRDS) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListRDSInstances(t *testing.T) {
	f := &fakeRDS{pages: []*rds.DescribeDBInstancesOutput{{DBInstances: []rdstypes.DBInstance{{
		DBInstanceIdentifier: aws.String("db1"),
		Engine:               aws.String("postgres"),
		DBInstanceClass:      aws.String("db.t3.micro"),
		DBInstanceStatus:     aws.String("available"),
		Endpoint:             &rdstypes.Endpoint{Address: aws.String("db1.abc.rds.amazonaws.com"), Port: aws.Int32(5432)},
	}}}}}
	out, err := listRDSInstances(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	d := out.Instances[0]
	if d.ID != "db1" || d.Engine != "postgres" || d.Class != "db.t3.micro" || d.Status != "available" || d.Endpoint != "db1.abc.rds.amazonaws.com:5432" {
		t.Fatalf("projection: %+v", d)
	}
}

func TestListRDSInstancesNoEndpoint(t *testing.T) {
	f := &fakeRDS{pages: []*rds.DescribeDBInstancesOutput{{DBInstances: []rdstypes.DBInstance{{
		DBInstanceIdentifier: aws.String("creating"),
		DBInstanceStatus:     aws.String("creating"),
	}}}}}
	out, err := listRDSInstances(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if out.Instances[0].Endpoint != "" {
		t.Fatalf("expected empty endpoint, got %q", out.Instances[0].Endpoint)
	}
}

// --- dynamodb --------------------------------------------------------------

type fakeDynamo struct {
	listPages []*dynamodb.ListTablesOutput
	listCalls int
	tables    map[string]*dynamodb.DescribeTableOutput
}

func (f *fakeDynamo) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	p := f.listPages[f.listCalls]
	f.listCalls++
	return p, nil
}

func (f *fakeDynamo) DescribeTable(_ context.Context, in *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return f.tables[aws.ToString(in.TableName)], nil
}

func TestListDynamoDBTables(t *testing.T) {
	f := &fakeDynamo{
		listPages: []*dynamodb.ListTablesOutput{{TableNames: []string{"orders"}}},
		tables: map[string]*dynamodb.DescribeTableOutput{
			"orders": {Table: &dynamodbtypes.TableDescription{
				TableName:      aws.String("orders"),
				TableStatus:    dynamodbtypes.TableStatusActive,
				ItemCount:      aws.Int64(42),
				TableSizeBytes: aws.Int64(1024),
			}},
		},
	}
	out, err := listDynamoDBTables(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	tb := out.Tables[0]
	if tb.Name != "orders" || tb.Status != "ACTIVE" || tb.ItemCount != 42 || tb.SizeBytes != 1024 {
		t.Fatalf("projection: %+v", tb)
	}
}
