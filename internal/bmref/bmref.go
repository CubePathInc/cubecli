// Package bmref handles the reference of a baremetal server: its uuid, which the
// API returns as "id" and takes in every /baremetal/{uuid} route. It exists next
// to cmdutil.IntPath, which VPS and networks keep using for plain numbers.
package bmref

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Parse validates a server reference typed by the user: a uuid (8-4-4-4-12 hex,
// any case), returned in lower case. Anything else, a server number included, is
// an error before any request.
func Parse(arg string) (string, error) {
	if uuidRe.MatchString(arg) {
		return strings.ToLower(arg), nil
	}
	return "", fmt.Errorf("invalid baremetal ID %q: baremetal reference must be a uuid", arg)
}

// Target validates the target of a route, BGP peer, load balancer target or
// alert when its type is baremetal, and returns it in lower case. Targets of
// other types are returned as given.
func Target(targetType, target string) (string, error) {
	if !strings.EqualFold(targetType, "baremetal") {
		return target, nil
	}
	return Parse(target)
}

// Path returns a cmdutil.PathFunc for routes keyed by a server uuid,
// e.g. Path("/baremetal/%s/protection").
func Path(format string) cmdutil.PathFunc {
	return func(arg string) (string, error) {
		ref, err := Parse(arg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(format, ref), nil
	}
}

// Matches reports whether the server with this id (its uuid) is the one the
// user named.
func Matches(ref, id string) bool {
	return ref != "" && strings.EqualFold(id, ref)
}
