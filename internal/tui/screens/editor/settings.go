package editor

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ilmars/netfu/internal/domain"
)

// validateCIDR accepts an empty address (DHCP profiles have none) or a
// valid ip/prefix.
func validateCIDR(value string) error {
	if value == "" {
		return nil
	}
	if _, err := netip.ParsePrefix(value); err != nil {
		return errors.New("not an ip/prefix, e.g. 192.168.1.10/24")
	}
	return nil
}

// str reads a string setting, tolerating a missing section or key.
func str(settings domain.ConnectionSettings, section, key string) string {
	if s, ok := settings[section][key].(string); ok {
		return s
	}
	return ""
}

func stringOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func boolOr(settings domain.ConnectionSettings, section, key string, fallback bool) bool {
	if b, ok := settings[section][key].(bool); ok {
		return b
	}
	return fallback
}

func stringList(settings domain.ConnectionSettings, section, key string) []string {
	switch v := settings[section][key].(type) {
	case []string:
		return v
	case []any:
		var list []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				list = append(list, s)
			}
		}
		return list
	}
	return nil
}

// firstAddress renders ipv4.address-data's first entry as CIDR notation.
func firstAddress(settings domain.ConnectionSettings) string {
	for _, entry := range anyList(settings["ipv4"]["address-data"]) {
		addr, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		address, _ := addr["address"].(string)
		if prefix, ok := toInt(addr["prefix"]); ok && address != "" {
			return fmt.Sprintf("%s/%d", address, prefix)
		}
	}
	return ""
}

// anyList flattens the slice shapes address-data arrives in — []any from
// the dbus round-trip, []map[string]any from fixtures.
func anyList(v any) []any {
	switch list := v.(type) {
	case []any:
		return list
	case []map[string]any:
		flat := make([]any, len(list))
		for i, item := range list {
			flat[i] = item
		}
		return flat
	}
	return nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint32:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

// ipv4MethodLabel maps NM's ipv4.method to the label the radio shows.
func ipv4MethodLabel(method string) string {
	switch method {
	case "manual":
		return "static"
	case "disabled":
		return "disabled"
	default:
		return "dhcp" // NM calls DHCP "auto", and it is the default
	}
}

func nmIPv4Method(label string) string {
	switch label {
	case "static":
		return "manual"
	case "disabled":
		return "disabled"
	default:
		return "auto"
	}
}

func ipv6MethodLabel(method string) string {
	switch method {
	case "manual":
		return "static"
	case "dhcp", "disabled":
		return method
	default:
		return "auto"
	}
}

func nmIPv6Method(label string) string {
	if label == "static" {
		return "manual"
	}
	return label
}

// addressData builds NM's ipv4.address-data from a CIDR string; the field
// validator has already guaranteed the shape.
func addressData(cidr string) []map[string]any {
	address, prefix, found := strings.Cut(cidr, "/")
	if !found {
		return nil
	}
	var n int
	fmt.Sscanf(prefix, "%d", &n)
	return []map[string]any{{"address": address, "prefix": n}}
}

func splitList(csv string) []string {
	var list []string
	for _, item := range strings.Split(csv, ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}
