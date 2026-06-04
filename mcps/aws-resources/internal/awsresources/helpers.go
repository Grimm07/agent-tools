package awsresources

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- list_profiles ---------------------------------------------------------

// ListProfilesInput takes no arguments; profiles come from the shared config.
type ListProfilesInput struct{}

// ListProfilesOutput is the structured result of list_profiles.
type ListProfilesOutput struct {
	Profiles []string `json:"profiles"`
	Source   string   `json:"source"`
}

// --- list_regions ----------------------------------------------------------

// describeRegionsAPI is the narrow slice of the EC2 client list_regions needs.
type describeRegionsAPI interface {
	DescribeRegions(context.Context, *ec2.DescribeRegionsInput, ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
}

// Region is a trimmed projection of an enabled AWS region.
type Region struct {
	Name        string `json:"name"`
	OptInStatus string `json:"opt_in_status"`
}

// ListRegionsInput selects the profile whose enabled regions to enumerate.
type ListRegionsInput struct {
	Profile string `json:"profile" jsonschema:"AWS profile name"`
	Region  string `json:"region,omitempty" jsonschema:"region to issue the DescribeRegions call from (optional; defaults to the profile's region)"`
}

// ListRegionsOutput is the structured result of list_regions.
type ListRegionsOutput struct {
	Regions []Region `json:"regions"`
}

// listRegions is the testable core: it takes the narrow API, not a client.
func listRegions(ctx context.Context, api describeRegionsAPI) (ListRegionsOutput, error) {
	var out ListRegionsOutput
	res, err := api.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return out, mapAWSError("ec2", err)
	}
	for _, r := range res.Regions {
		out.Regions = append(out.Regions, Region{
			Name:        aws.ToString(r.RegionName),
			OptInStatus: aws.ToString(r.OptInStatus),
		})
	}
	return out, nil
}

// registerHelperTools registers list_profiles and list_regions.
func registerHelperTools(s *mcp.Server, cache *configCache) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_profiles",
		Description: "List AWS profile names from the shared config file (~/.aws/config). Names only; no credentials.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListProfilesInput) (*mcp.CallToolResult, ListProfilesOutput, error) {
		path := defaultConfigPath()
		profiles, err := listProfilesFrom(path)
		if err != nil {
			return nil, ListProfilesOutput{}, err
		}
		out := ListProfilesOutput{Profiles: profiles, Source: path}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_regions",
		Description: "List enabled AWS regions for a profile via EC2 DescribeRegions.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListRegionsInput) (*mcp.CallToolResult, ListRegionsOutput, error) {
		cfg, err := cache.get(ctx, in.Profile, in.Region)
		if err != nil {
			return nil, ListRegionsOutput{}, err
		}
		out, err := listRegions(ctx, ec2.NewFromConfig(cfg))
		if err != nil {
			return nil, ListRegionsOutput{}, err
		}
		return textResult(out), out, nil
	})
}
