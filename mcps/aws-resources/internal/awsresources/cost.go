package awsresources

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const costMetric = "UnblendedCost"

// getCostAndUsageAPI is the narrow slice of the Cost Explorer client
// get_cost_summary needs.
type getCostAndUsageAPI interface {
	GetCostAndUsage(context.Context, *costexplorer.GetCostAndUsageInput, ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

// ServiceCost is the cost attributed to a single AWS service.
type ServiceCost struct {
	Service string `json:"service"`
	Amount  string `json:"amount"`
}

// GetCostInput selects the profile and an optional date window. Cost Explorer is
// a global endpoint, so no region. start/end are YYYY-MM-DD; End is exclusive.
type GetCostInput struct {
	Profile string `json:"profile" jsonschema:"AWS profile name"`
	Start   string `json:"start,omitempty" jsonschema:"start date YYYY-MM-DD (default: first of current month)"`
	End     string `json:"end,omitempty" jsonschema:"end date YYYY-MM-DD, exclusive (default: tomorrow)"`
}

// GetCostOutput is the structured result of get_cost_summary.
type GetCostOutput struct {
	Start    string        `json:"start"`
	End      string        `json:"end"`
	Unit     string        `json:"unit"`
	Total    string        `json:"total"`
	Services []ServiceCost `json:"services"`
}

// costPeriod resolves the requested window, defaulting to month-to-date: the
// first of the current month through tomorrow (Cost Explorer's End is
// exclusive, so "tomorrow" includes today's partial usage). now is injected for
// deterministic testing.
func costPeriod(start, end string, now time.Time) (string, string) {
	if start == "" {
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	if end == "" {
		end = now.UTC().AddDate(0, 0, 1).Format("2006-01-02")
	}
	return start, end
}

// getCostSummary fetches month-to-date unblended cost grouped by service. It is
// the only tool that incurs an AWS charge (Cost Explorer bills ~$0.01 per
// request); callers invoke it deliberately.
func getCostSummary(ctx context.Context, api getCostAndUsageAPI, start, end string) (GetCostOutput, error) {
	out := GetCostOutput{Start: start, End: end, Unit: "USD", Total: "0.00"}
	res, err := api.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &cetypes.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{costMetric},
		GroupBy: []cetypes.GroupDefinition{{
			Type: cetypes.GroupDefinitionTypeDimension,
			Key:  aws.String("SERVICE"),
		}},
	})
	if err != nil {
		return GetCostOutput{}, mapAWSError("ce", err)
	}

	var total float64
	for _, r := range res.ResultsByTime {
		for _, g := range r.Groups {
			service := ""
			if len(g.Keys) > 0 {
				service = g.Keys[0]
			}
			mv, ok := g.Metrics[costMetric]
			if !ok {
				continue
			}
			amount := aws.ToString(mv.Amount)
			if u := aws.ToString(mv.Unit); u != "" {
				out.Unit = u
			}
			if f, perr := strconv.ParseFloat(amount, 64); perr == nil {
				total += f
			}
			out.Services = append(out.Services, ServiceCost{Service: service, Amount: amount})
		}
	}
	out.Total = strconv.FormatFloat(total, 'f', 2, 64)
	return out, nil
}

// registerCostTools registers get_cost_summary.
func registerCostTools(s *mcp.Server, cache *configCache) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_cost_summary",
		Description: "Month-to-date unblended cost by service via Cost Explorer. WARNING: each call is a paid AWS request (~$0.01).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetCostInput) (*mcp.CallToolResult, GetCostOutput, error) {
		cfg, err := cache.get(ctx, in.Profile, "")
		if err != nil {
			return nil, GetCostOutput{}, err
		}
		start, end := costPeriod(in.Start, in.End, time.Now())
		out, err := getCostSummary(ctx, costexplorer.NewFromConfig(cfg), start, end)
		if err != nil {
			return nil, GetCostOutput{}, err
		}
		return textResult(out), out, nil
	})
}
