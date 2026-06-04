package awsresources

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

type fakeCostExplorer struct {
	out    *costexplorer.GetCostAndUsageOutput
	err    error
	lastIn *costexplorer.GetCostAndUsageInput
}

func (f *fakeCostExplorer) GetCostAndUsage(_ context.Context, in *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	f.lastIn = in
	return f.out, f.err
}

func TestGetCostSummary(t *testing.T) {
	f := &fakeCostExplorer{out: &costexplorer.GetCostAndUsageOutput{
		ResultsByTime: []cetypes.ResultByTime{{
			TimePeriod: &cetypes.DateInterval{Start: aws.String("2024-06-01"), End: aws.String("2024-06-15")},
			Groups: []cetypes.Group{
				{Keys: []string{"Amazon EC2"}, Metrics: map[string]cetypes.MetricValue{"UnblendedCost": {Amount: aws.String("12.34"), Unit: aws.String("USD")}}},
				{Keys: []string{"Amazon S3"}, Metrics: map[string]cetypes.MetricValue{"UnblendedCost": {Amount: aws.String("1.66"), Unit: aws.String("USD")}}},
			},
		}},
	}}
	out, err := getCostSummary(context.Background(), f, "2024-06-01", "2024-06-15")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Services) != 2 {
		t.Fatalf("got %d services", len(out.Services))
	}
	if out.Services[0].Service != "Amazon EC2" || out.Services[0].Amount != "12.34" {
		t.Fatalf("service0: %+v", out.Services[0])
	}
	if out.Unit != "USD" {
		t.Fatalf("unit = %q", out.Unit)
	}
	if out.Total != "14.00" {
		t.Fatalf("total = %q want 14.00", out.Total)
	}
	if out.Start != "2024-06-01" || out.End != "2024-06-15" {
		t.Fatalf("period: %s..%s", out.Start, out.End)
	}
	// Verify the request shape: monthly granularity, unblended metric, grouped
	// by SERVICE dimension.
	if f.lastIn.Granularity != cetypes.GranularityMonthly {
		t.Errorf("granularity = %v", f.lastIn.Granularity)
	}
	if len(f.lastIn.Metrics) != 1 || f.lastIn.Metrics[0] != "UnblendedCost" {
		t.Errorf("metrics = %v", f.lastIn.Metrics)
	}
	if len(f.lastIn.GroupBy) != 1 || aws.ToString(f.lastIn.GroupBy[0].Key) != "SERVICE" || f.lastIn.GroupBy[0].Type != cetypes.GroupDefinitionTypeDimension {
		t.Errorf("groupby = %+v", f.lastIn.GroupBy)
	}
}

// When start/end are empty, costPeriod defaults to month-to-date (first of the
// current month through tomorrow, since the CE End date is exclusive).
func TestCostPeriodDefaultsToMTD(t *testing.T) {
	now := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	start, end := costPeriod("", "", now)
	if start != "2024-06-01" {
		t.Errorf("start = %q want 2024-06-01", start)
	}
	if end != "2024-06-16" {
		t.Errorf("end = %q want 2024-06-16 (exclusive, tomorrow)", end)
	}
}

func TestCostPeriodHonorsExplicitDates(t *testing.T) {
	start, end := costPeriod("2024-01-01", "2024-02-01", time.Now())
	if start != "2024-01-01" || end != "2024-02-01" {
		t.Errorf("got %s..%s", start, end)
	}
}

func TestGetCostSummaryEmpty(t *testing.T) {
	f := &fakeCostExplorer{out: &costexplorer.GetCostAndUsageOutput{}}
	out, err := getCostSummary(context.Background(), f, "2024-06-01", "2024-06-15")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Services) != 0 || out.Total != "0.00" {
		t.Fatalf("expected empty summary, got %+v", out)
	}
}
