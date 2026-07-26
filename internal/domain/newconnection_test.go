package domain

import (
	"strings"
	"testing"
)

func TestVPN_AddConnectionOffersNoVPNTypeWithExplainer(t *testing.T) {
	types := CreatableConnectionTypes()
	if len(types) == 0 {
		t.Fatal("the new-connection picker should offer at least one type")
	}
	for _, ct := range types {
		if ct == "vpn" {
			t.Errorf("vpn must not be offered as a creatable type, got %v", types)
		}
	}
	for _, want := range []string{"D-Bus", "nmcli"} {
		if !strings.Contains(VPNNotCreatableExplainer, want) {
			t.Errorf("the explainer should mention %q, got: %s", want, VPNNotCreatableExplainer)
		}
	}
}
