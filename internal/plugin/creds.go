package plugin

import (
	"os"
	"strings"
)

// ResolveCredentials resolves credentials_ref.
// Formats:
//   - empty: empty map
//   - "env:VAR" or "VAR": value from environment
//   - "file:/path": file contents as {"token": content}
//   - "env:USER,env:PASS": multiple keys named after var suffix
func ResolveCredentials(ref string) Credentials {
	out := Credentials{}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return out
	}
	parts := strings.Split(ref, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "file:") {
			path := strings.TrimPrefix(p, "file:")
			b, err := os.ReadFile(path)
			if err == nil {
				out["token"] = strings.TrimSpace(string(b))
			}
			continue
		}
		name := strings.TrimPrefix(p, "env:")
		if v, ok := os.LookupEnv(name); ok {
			key := name
			if i := strings.LastIndex(name, "_"); i >= 0 && i+1 < len(name) {
				key = strings.ToLower(name[i+1:])
			}
			out[key] = v
			out[name] = v
		}
	}
	return out
}
