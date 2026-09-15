package analyzer

import "strings"

// NormalizeHandlerID converts a runtime function name into the stable identifier
// used by generated schema registrations.
func NormalizeHandlerID(name string) string {
	name = strings.TrimSuffix(name, "-fm")
	if marker := strings.Index(name, ".func"); marker >= 0 {
		name = name[:marker]
	}
	start := strings.LastIndex(name, ".(")
	end := strings.LastIndex(name, ").")
	if start >= 0 && end > start {
		pkg := name[:start]
		receiver := strings.TrimPrefix(name[start+2:end], "*")
		method := name[end+2:]
		return pkg + "." + receiver + "." + method
	}
	return name
}
