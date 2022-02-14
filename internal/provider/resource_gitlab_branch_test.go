package provider

import (
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	gitlab "github.com/xanzy/go-gitlab"
	"testing"
)

func TestAccGitlabBranch_basic(t *testing.T) {
	var branch gitlab.Branch
	var branch2 gitlab.Branch
	rInt := acctest.RandInt()
	fooBranchName := fmt.Sprintf("testbranch-%d", rInt)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckGitlabBranchDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGitlabBranchConfig(rInt),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGitlabBranchExists("foo", "test", &branch, rInt),
					testAccCheckGitlabBranchExists("foo2", "test", &branch2, rInt),
					testAccCheckGitlabBranchAttributes("foo", "test", &branch, &testAccGitlabBranchExpectedAttributes{
						Name:    fmt.Sprintf("testbranch-%d", rInt),
						Ref:     "main",
						CanPush: true,
						Commit:  true,
					}),
					testAccCheckGitlabBranchAttributes("foo2", "test", &branch2, &testAccGitlabBranchExpectedAttributes{
						Name:    fmt.Sprintf("testbranch2-%d", rInt),
						Ref:     fmt.Sprintf("testbranch-%d", rInt),
						CanPush: true,
						Commit:  true,
					}),
					testAccCheckGitlabBranchRef("foo", "main"),
					testAccCheckGitlabBranchRef("foo2", fooBranchName),
					testAccCheckGitlabBranchCommit("foo2"),
				),
			},
			// Test ImportState
			{
				ResourceName:      "gitlab_branch.foo",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckGitlabBranchCommit(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs := s.RootModule().Resources[fmt.Sprintf("gitlab_branch.%s", n)]
		commit := rs.Primary.Attributes["commit.0.id"]
		if commit == "" {
			return fmt.Errorf("expected commit to be populated")
		}
		return nil
	}
}

func testAccCheckGitlabBranchRef(n, expectedRef string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs := s.RootModule().Resources[fmt.Sprintf("gitlab_branch.%s", n)]
		ref := rs.Primary.Attributes["ref"]
		if ref != expectedRef {
			return fmt.Errorf("expected ref: %s got: %s", expectedRef, ref)
		}
		return nil
	}
}

func testAccCheckGitlabBranchDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "gitlab_branch" {
			continue
		}
		name := rs.Primary.Attributes["name"]
		project := rs.Primary.Attributes["project"]
		branch, resp, err := testGitlabClient.Branches.GetBranch(project, name)
		if err == nil {
			if branch != nil && branch.Name == name {
				return fmt.Errorf("Branch still exists")
			}
		}
		if resp.StatusCode != 404 {
			return err
		}
		return nil
	}
	return nil
}

func testAccCheckGitlabBranchAttributes(n, p string, branch *gitlab.Branch, want *testAccGitlabBranchExpectedAttributes) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[fmt.Sprintf("gitlab_branch.%s", n)]
		if !ok {
			return fmt.Errorf("Not Found: %s", n)
		}

		_, branchName, err := parseTwoPartID(rs.Primary.ID)
		if err != nil {
			return errors.New("Error in splitting project and branch IDs")
		}
		if branch.WebURL == "" {
			return errors.New("got empty web url")
		}
		if branchName != want.Name || branchName != branch.Name {
			return fmt.Errorf("got name %s; want %s", branch.Name, want.Name)
		}
		if want.Commit {
			if branch.Commit == nil {
				return errors.New("Branch commit is nil but expected to be populated")
			}
			if branch.Commit.ID == "" {
				return errors.New("Commit has an empty ID")
			}
		} else {
			if branch.Commit != nil {
				return fmt.Errorf("Unexpected commit %v", branch.Commit)
			}
		}
		if branch.CanPush != want.CanPush {
			return fmt.Errorf("can push %t; want %t", branch.CanPush, want.CanPush)
		}
		if branch.Default != want.Default {
			return fmt.Errorf("Default %t; want %t", branch.Default, want.Default)
		}
		if branch.Merged != want.Merged {
			return fmt.Errorf("Merged %t; want %t", branch.Merged, want.Merged)
		}
		return nil
	}
}

func testAccCheckGitlabBranchExists(n, p string, branch *gitlab.Branch, rInt int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[fmt.Sprintf("gitlab_branch.%s", n)]
		if !ok {
			return fmt.Errorf("Not Found: %s", n)
		}
		pid, name, err := parseTwoPartID(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("Error in splitting project and branch IDs")
		}
		gotBranch, _, err := testGitlabClient.Branches.GetBranch(pid, name)
		if err != nil {
			return err
		}
		*branch = *gotBranch
		return err
	}
}

func testAccGitlabBranchConfig(rInt int) string {
	return fmt.Sprintf(`
	resource "gitlab_project" "test" {
		name = "foo-%[1]d"
		description = "Terraform acceptance tests"
	  
		# So that acceptance tests can be run in a gitlab organization
		# with no billing
		visibility_level = "public"
	}
	resource "gitlab_branch" "foo" {
		name = "testbranch-%[1]d"
		ref = "main"
		project = gitlab_project.test.id
	}
	resource "gitlab_branch" "foo2" {
		name = "testbranch2-%[1]d"
		ref = gitlab_branch.foo.name
		project = gitlab_project.test.id
	}
  `, rInt)
}

type testAccGitlabBranchExpectedAttributes struct {
	Name    string
	WebURL  string
	CanPush bool
	Default bool
	Merged  bool
	Ref     string
	Project string
	Commit  bool
}
