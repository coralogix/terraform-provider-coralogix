package acctest

import (
	"context"
	"os"
	"testing"

	ipaccess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ip_access_service"
)

const (
	ipAccessAck      = "IP_ACCESS_GENERATED_LIVE_ALLOW_MUTATION"
	ipAccessAckValue = "I_UNDERSTAND_THIS_USES_A_DISPOSABLE_TEAM"
)

// RequireDisposableIpAccessTeam stops a test that would replace the IP access settings of the team,
// unless the settings are safe to overwrite. The resource holds the settings of the whole team, and
// its delete removes them, so a run against a team that has an allowlist would destroy it. The test
// needs the explicit acknowledgement of the handwritten live test, and a team that has no entry and
// keeps support access disabled. configs is unused: the guard does not depend on the config.
func RequireDisposableIpAccessTeam(t *testing.T, _ ...string) {
	t.Helper()
	if os.Getenv(ipAccessAck) != ipAccessAckValue {
		t.Fatalf("set %s=%s only when the API key belongs to a disposable team: this test replaces the IP access settings of the team and deletes them", ipAccessAck, ipAccessAckValue)
	}
	clients, err := newClientSet()
	if err != nil {
		t.Fatal(err)
	}
	out, httpResp, err := clients.IpAccess().IpAccessServiceGetCompanyIpAccessSettings(context.Background()).Execute()
	if err != nil {
		status := 0
		if httpResp != nil {
			status = httpResp.StatusCode
		}
		t.Fatalf("preflight read of the IP access settings failed (HTTP %d): %v; refusing to modify the team", status, err)
	}
	settings := out.GetSettings()
	if len(settings.GetIpAccess()) != 0 || settings.GetEnableCoralogixCustomerSupportAccess() != ipaccess.CORALOGIXCUSTOMERSUPPORTACCESS_CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED {
		t.Fatalf("preflight found IP access entries or support access that is not disabled; refusing to overwrite the settings of this team")
	}
}
