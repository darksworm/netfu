package domain

import "sort"

type Security string

const (
	SecurityOpen       Security = "open"
	SecurityWEP        Security = "wep"
	SecurityWPA        Security = "wpa"
	SecurityWPA2       Security = "wpa2"
	SecurityWPA3       Security = "wpa3"
	SecurityWPA2WPA3   Security = "wpa2-wpa3"
	SecurityEnterprise Security = "802.1x"
)

// Bits of NM's 80211ApFlags / 80211ApSecurityFlags that decide
// classification (verified against networkmanager.dev nm-dbus-types).
const (
	nmAPFlagPrivacy     uint32 = 0x1
	nmAPSecKeyMgmtPSK   uint32 = 0x100
	nmAPSecKeyMgmt8021X uint32 = 0x200
	nmAPSecKeyMgmtSAE   uint32 = 0x400
)

// ClassifySecurity maps an AP's raw NM flag words to the badge shown in the list.
func ClassifySecurity(flags, wpaFlags, rsnFlags uint32) Security {
	hasPSK := rsnFlags&nmAPSecKeyMgmtPSK != 0
	hasSAE := rsnFlags&nmAPSecKeyMgmtSAE != 0
	switch {
	case (wpaFlags|rsnFlags)&nmAPSecKeyMgmt8021X != 0:
		return SecurityEnterprise
	case hasPSK && hasSAE:
		return SecurityWPA2WPA3
	case hasSAE:
		return SecurityWPA3
	case hasPSK:
		return SecurityWPA2
	case wpaFlags&nmAPSecKeyMgmtPSK != 0:
		return SecurityWPA
	case flags&nmAPFlagPrivacy != 0:
		return SecurityWEP
	}
	return SecurityOpen
}

// WifiList is the wifi tab's row model: in-range rows in display order,
// plus saved networks with no matching scanned AP.
type WifiList struct {
	InRange    []AccessPoint
	OutOfRange []string
}

// BuildWifiList orders deduped APs into stable display buckets:
// active, connecting, saved-in-range by signal, unsaved by signal.
// Saved SSIDs with no scanned AP land in OutOfRange.
func BuildWifiList(aps []AccessPoint, savedSSIDs []string, activeSSID, connectingSSID string) WifiList {
	rows := make([]AccessPoint, len(aps))
	copy(rows, aps)
	saved := map[string]bool{}
	for _, s := range savedSSIDs {
		saved[s] = true
	}
	bucket := func(ap AccessPoint) int {
		switch {
		case activeSSID != "" && ap.SSID == activeSSID:
			return 0
		case connectingSSID != "" && ap.SSID == connectingSSID:
			return 1
		case saved[ap.SSID]:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		bi, bj := bucket(rows[i]), bucket(rows[j])
		if bi != bj {
			return bi < bj
		}
		return rows[i].Strength > rows[j].Strength
	})
	inRange := map[string]bool{}
	for _, ap := range aps {
		inRange[ap.SSID] = true
	}
	var outOfRange []string
	for _, s := range savedSSIDs {
		if !inRange[s] {
			outOfRange = append(outOfRange, s)
		}
	}
	return WifiList{InRange: rows, OutOfRange: outOfRange}
}

// DedupeAPs collapses APs sharing an SSID to the strongest BSSID,
// preserving first-seen order. Hidden APs (empty SSID) are distinct
// networks, so each keeps its own row.
func DedupeAPs(aps []AccessPoint) []AccessPoint {
	var out []AccessPoint
	index := map[string]int{}
	for _, ap := range aps {
		if ap.SSID == "" {
			out = append(out, ap)
			continue
		}
		i, seen := index[ap.SSID]
		if !seen {
			index[ap.SSID] = len(out)
			out = append(out, ap)
			continue
		}
		if ap.Strength > out[i].Strength {
			out[i] = ap
		}
	}
	return out
}
