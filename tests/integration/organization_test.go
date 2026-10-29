//go:build integration

package integration

import (
	"net/http"
	"testing"
)

func TestStaffCannotUpdateEventOfOtherOrganization(t *testing.T) {
	ownerA := registerUser(t)
	organizationA := createOrg(t, ownerA.AccessToken)
	staffA := registerUser(t)
	addStaff(t, ownerA.AccessToken, organizationA, staffA.Email)
	eventA := createEvent(t, ownerA.AccessToken, organizationA, 10)

	ownerB := registerUser(t)
	organizationB := createOrg(t, ownerB.AccessToken)
	eventB := createEvent(t, ownerB.AccessToken, organizationB, 10)

	rename := map[string]string{"name": "Renamed"}

	// Without this, a 403 below could mean Staff are blocked everywhere, not just outside their org.
	status := doJSON(t, http.MethodPatch, "/events/"+eventA, staffA.AccessToken, rename, nil)
	if status != http.StatusOK {
		t.Fatalf("staff of org A updating org A's event: status %d, want %d", status, http.StatusOK)
	}

	status = doJSON(t, http.MethodPatch, "/events/"+eventB, staffA.AccessToken, rename, nil)
	if status != http.StatusForbidden {
		t.Fatalf("staff of org A updating org B's event: status %d, want %d", status, http.StatusForbidden)
	}
}
