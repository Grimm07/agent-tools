package awsresources

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IAM tools return metadata only — names, IDs, dates, attachment counts, and a
// short trust summary. Policy documents, access keys, and any secret material
// are never read or returned.

// ListIAMInput selects the profile. IAM is global, so there is no region.
type ListIAMInput struct {
	Profile  string `json:"profile" jsonschema:"AWS profile name"`
	MaxItems int    `json:"max_items,omitempty" jsonschema:"max results (default 100, cap 1000)"`
}

// --- list_iam_users --------------------------------------------------------

// IAMUser is a metadata-only projection of an IAM user.
type IAMUser struct {
	Name             string `json:"name"`
	ID               string `json:"id"`
	Created          string `json:"created"`
	PasswordLastUsed string `json:"password_last_used"`
}

// ListIAMUsersOutput is the structured result of list_iam_users.
type ListIAMUsersOutput struct {
	Users     []IAMUser `json:"users"`
	Truncated bool      `json:"truncated"`
}

// listIAMUsers walks IAM user pages up to max.
func listIAMUsers(ctx context.Context, api iam.ListUsersAPIClient, max int) (ListIAMUsersOutput, error) {
	var out ListIAMUsersOutput
	p := iam.NewListUsersPaginator(api, &iam.ListUsersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListIAMUsersOutput{}, mapAWSError("iam", err)
		}
		for _, u := range page.Users {
			if len(out.Users) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Users = append(out.Users, IAMUser{
				Name:             aws.ToString(u.UserName),
				ID:               aws.ToString(u.UserId),
				Created:          formatTime(u.CreateDate),
				PasswordLastUsed: formatTime(u.PasswordLastUsed),
			})
		}
	}
	return out, nil
}

// --- list_iam_roles --------------------------------------------------------

// IAMRole is a metadata-only projection of an IAM role. Trust is a short
// human-readable summary (path + description) — the assume-role policy document
// itself is never decoded or returned.
type IAMRole struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Created string `json:"created"`
	Trust   string `json:"trust"`
}

// ListIAMRolesOutput is the structured result of list_iam_roles.
type ListIAMRolesOutput struct {
	Roles     []IAMRole `json:"roles"`
	Truncated bool      `json:"truncated"`
}

// listIAMRoles walks IAM role pages up to max.
func listIAMRoles(ctx context.Context, api iam.ListRolesAPIClient, max int) (ListIAMRolesOutput, error) {
	var out ListIAMRolesOutput
	p := iam.NewListRolesPaginator(api, &iam.ListRolesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListIAMRolesOutput{}, mapAWSError("iam", err)
		}
		for _, r := range page.Roles {
			if len(out.Roles) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Roles = append(out.Roles, IAMRole{
				Name:    aws.ToString(r.RoleName),
				ID:      aws.ToString(r.RoleId),
				Created: formatTime(r.CreateDate),
				Trust:   trustSummary(r),
			})
		}
	}
	return out, nil
}

// trustSummary builds a short, non-sensitive description of a role from its path
// and description. It deliberately ignores AssumeRolePolicyDocument so no policy
// document content is exposed.
func trustSummary(r iamtypes.Role) string {
	parts := make([]string, 0, 2)
	if p := aws.ToString(r.Path); p != "" {
		parts = append(parts, p)
	}
	if d := aws.ToString(r.Description); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, " ")
}

// --- list_iam_policies -----------------------------------------------------

// IAMPolicy is a metadata-only projection of a customer-managed IAM policy. The
// policy document is never read or returned.
type IAMPolicy struct {
	Name            string `json:"name"`
	ARN             string `json:"arn"`
	AttachmentCount int32  `json:"attachment_count"`
}

// ListIAMPoliciesOutput is the structured result of list_iam_policies.
type ListIAMPoliciesOutput struct {
	Policies  []IAMPolicy `json:"policies"`
	Truncated bool        `json:"truncated"`
}

// listIAMPolicies walks customer-managed (Local scope) IAM policy pages up to
// max. AWS-managed policies are excluded.
func listIAMPolicies(ctx context.Context, api iam.ListPoliciesAPIClient, max int) (ListIAMPoliciesOutput, error) {
	var out ListIAMPoliciesOutput
	p := iam.NewListPoliciesPaginator(api, &iam.ListPoliciesInput{Scope: iamtypes.PolicyScopeTypeLocal})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return ListIAMPoliciesOutput{}, mapAWSError("iam", err)
		}
		for _, pol := range page.Policies {
			if len(out.Policies) >= max {
				out.Truncated = true
				return out, nil
			}
			out.Policies = append(out.Policies, IAMPolicy{
				Name:            aws.ToString(pol.PolicyName),
				ARN:             aws.ToString(pol.Arn),
				AttachmentCount: aws.ToInt32(pol.AttachmentCount),
			})
		}
	}
	return out, nil
}

// registerIdentityTools registers the IAM (identity) tools. All are global
// (profile only) and return metadata only.
func registerIdentityTools(s *mcp.Server, cache *configCache) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_iam_users",
		Description: "List IAM users for a profile (name, id, created, password_last_used). Metadata only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListIAMInput) (*mcp.CallToolResult, ListIAMUsersOutput, error) {
		cfg, err := cache.get(ctx, in.Profile, "")
		if err != nil {
			return nil, ListIAMUsersOutput{}, err
		}
		out, err := listIAMUsers(ctx, iam.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListIAMUsersOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_iam_roles",
		Description: "List IAM roles for a profile (name, id, created, trust summary). Metadata only; no policy documents.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListIAMInput) (*mcp.CallToolResult, ListIAMRolesOutput, error) {
		cfg, err := cache.get(ctx, in.Profile, "")
		if err != nil {
			return nil, ListIAMRolesOutput{}, err
		}
		out, err := listIAMRoles(ctx, iam.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListIAMRolesOutput{}, err
		}
		return textResult(out), out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_iam_policies",
		Description: "List customer-managed IAM policies for a profile (name, arn, attachment_count). Metadata only; no policy documents.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListIAMInput) (*mcp.CallToolResult, ListIAMPoliciesOutput, error) {
		cfg, err := cache.get(ctx, in.Profile, "")
		if err != nil {
			return nil, ListIAMPoliciesOutput{}, err
		}
		out, err := listIAMPolicies(ctx, iam.NewFromConfig(cfg), clampMax(in.MaxItems))
		if err != nil {
			return nil, ListIAMPoliciesOutput{}, err
		}
		return textResult(out), out, nil
	})
}
