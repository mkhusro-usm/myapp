package github

import "testing"

func TestOrg(t *testing.T) {
	client, _ := setupTestClient(t)
	if got := client.Org(); got != "test-org" {
		t.Errorf("Org() = %q, want %q", got, "test-org")
	}
}

func TestLogRateLimitNilResponse(t *testing.T) {
	logRateLimit(nil)
}
