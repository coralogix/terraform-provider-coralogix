package provider

import (
	"fmt"
	"testing"

	usersservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// The two corner cases of allowed_login_mode that the generated lifecycle does not
// reach, live against the API:
//
//   - a configuration that lists both modes with SSO first and repeats one. The
//     backend answers each distinct mode once, local before SSO; the read has to
//     reconcile that answer with the configuration, and the plan after the apply
//     has to stay empty.
//   - the explicit empty list that clears the modes. The attribute is
//     Optional+Computed, so removing it from the configuration keeps the backend
//     values; only the empty list clears them.
func TestAccCoralogixResourceUserLoginModesReversedAndCleared(t *testing.T) {
	userName := randUserName()
	sso := usersservice.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_SSO
	local := usersservice.ALLOWEDLOGINMODE_ALLOWED_LOGIN_MODE_LOCAL

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceUserLoginModes(userName, `["ALLOWED_LOGIN_MODE_SSO", "ALLOWED_LOGIN_MODE_LOCAL", "ALLOWED_LOGIN_MODE_SSO"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "allowed_login_mode.#", "3"),
					resource.TestCheckResourceAttr(userResourceName, "allowed_login_mode.0", "ALLOWED_LOGIN_MODE_SSO"),
					resource.TestCheckResourceAttr(userResourceName, "allowed_login_mode.1", "ALLOWED_LOGIN_MODE_LOCAL"),
					resource.TestCheckResourceAttr(userResourceName, "allowed_login_mode.2", "ALLOWED_LOGIN_MODE_SSO"),
					// The wire carries the configuration as sent; the backend folds it.
					testAccCheckUserLoginModes(userName, sso, local),
				),
			},
			{
				Config: testAccCoralogixResourceUserLoginModes(userName, `[]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(userResourceName, "allowed_login_mode.#", "0"),
					testAccCheckUserLoginModes(userName),
				),
			},
		},
	})
}

func testAccCoralogixResourceUserLoginModes(userName, modes string) string {
	return fmt.Sprintf(`resource "coralogix_user" "test" {
  allowed_login_mode = %s
  user_name = %q
}`, modes, userName)
}
