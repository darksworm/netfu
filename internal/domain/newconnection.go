package domain

// CreatableConnectionTypes is what a new-connection picker may offer. VPN is
// deliberately absent: NM's D-Bus API cannot create VPN connections, so netfu
// only activates existing profiles (see VPNNotCreatableExplainer).
func CreatableConnectionTypes() []string {
	return []string{"802-3-ethernet", "802-11-wireless"}
}

const VPNNotCreatableExplainer = "VPN profiles can't be created over D-Bus — import one with nmcli or your VPN's importer, then activate it here."
