package awsresources

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// --- iam users -------------------------------------------------------------

type fakeIAMUsers struct {
	pages []*iam.ListUsersOutput
	calls int
}

func (f *fakeIAMUsers) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListIAMUsers(t *testing.T) {
	created := time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC)
	used := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeIAMUsers{pages: []*iam.ListUsersOutput{{Users: []iamtypes.User{{
		UserName:         aws.String("alice"),
		UserId:           aws.String("AIDA1"),
		CreateDate:       aws.Time(created),
		PasswordLastUsed: aws.Time(used),
	}}}}}
	out, err := listIAMUsers(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	u := out.Users[0]
	if u.Name != "alice" || u.ID != "AIDA1" || u.Created != "2023-05-01T00:00:00Z" || u.PasswordLastUsed != "2024-06-01T00:00:00Z" {
		t.Fatalf("projection: %+v", u)
	}
}

func TestListIAMUsersNoPasswordUse(t *testing.T) {
	f := &fakeIAMUsers{pages: []*iam.ListUsersOutput{{Users: []iamtypes.User{{
		UserName: aws.String("svc"), UserId: aws.String("AIDA2"), CreateDate: aws.Time(time.Now()),
	}}}}}
	out, err := listIAMUsers(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	if out.Users[0].PasswordLastUsed != "" {
		t.Fatalf("expected empty PasswordLastUsed, got %q", out.Users[0].PasswordLastUsed)
	}
}

// --- iam roles -------------------------------------------------------------

type fakeIAMRoles struct {
	pages []*iam.ListRolesOutput
	calls int
}

func (f *fakeIAMRoles) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListIAMRoles(t *testing.T) {
	created := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeIAMRoles{pages: []*iam.ListRolesOutput{{Roles: []iamtypes.Role{{
		RoleName:                 aws.String("admin"),
		RoleId:                   aws.String("AROA1"),
		CreateDate:               aws.Time(created),
		Path:                     aws.String("/"),
		Description:              aws.String("admin role"),
		AssumeRolePolicyDocument: aws.String("%7B%22should%22%3A%22not-leak%22%7D"),
	}}}}}
	out, err := listIAMRoles(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	r := out.Roles[0]
	if r.Name != "admin" || r.ID != "AROA1" || r.Created != "2022-01-01T00:00:00Z" || r.Trust != "/ admin role" {
		t.Fatalf("projection: %+v", r)
	}
	// The role's trust policy document must never appear in the projection.
	if r.Trust == aws.ToString(f.pages[0].Roles[0].AssumeRolePolicyDocument) {
		t.Fatal("trust policy document leaked into projection")
	}
}

// --- iam policies ----------------------------------------------------------

type fakeIAMPolicies struct {
	pages  []*iam.ListPoliciesOutput
	calls  int
	lastIn *iam.ListPoliciesInput
}

func (f *fakeIAMPolicies) ListPolicies(_ context.Context, in *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	f.lastIn = in
	p := f.pages[f.calls]
	f.calls++
	return p, nil
}

func TestListIAMPolicies(t *testing.T) {
	f := &fakeIAMPolicies{pages: []*iam.ListPoliciesOutput{{Policies: []iamtypes.Policy{{
		PolicyName:      aws.String("ReadOnly"),
		Arn:             aws.String("arn:aws:iam::1:policy/ReadOnly"),
		AttachmentCount: aws.Int32(3),
	}}}}}
	out, err := listIAMPolicies(context.Background(), f, 100)
	if err != nil {
		t.Fatal(err)
	}
	p := out.Policies[0]
	if p.Name != "ReadOnly" || p.ARN != "arn:aws:iam::1:policy/ReadOnly" || p.AttachmentCount != 3 {
		t.Fatalf("projection: %+v", p)
	}
	// Must scope to customer-managed (Local) policies only.
	if f.lastIn == nil || f.lastIn.Scope != iamtypes.PolicyScopeTypeLocal {
		t.Fatalf("expected Scope=Local, got %v", f.lastIn.Scope)
	}
}
