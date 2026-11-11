// Package bmref handles the reference of a baremetal server: the number or the
// uuid the API accepts in every /baremetal/{ref} route. It exists next to
// cmdutil.IntPath, which VPS and networks keep using for plain numbers.
package bmref

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
)

var (
	numberRe = regexp.MustCompile(`^[0-9]+$`)
	uuidRe   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// Parse validates a server reference typed by the user: digits only (the
// number) or a uuid (8-4-4-4-12 hex, any case). A number is returned as given
// and a uuid in lower case. Anything else is an error before any request.
func Parse(arg string) (string, error) {
	if numberRe.MatchString(arg) {
		return arg, nil
	}
	if uuidRe.MatchString(arg) {
		return strings.ToLower(arg), nil
	}
	return "", fmt.Errorf("invalid baremetal ID: %q (use the server number or its uuid)", arg)
}

// Path returns a cmdutil.PathFunc for routes keyed by a server reference,
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

// FlexID decodes the "id" of a server whether the API sends a number or a
// string, and prints back as a number when it is one, so scripts reading
// `jq '.id == 123'` keep working.
type FlexID string

func (f *FlexID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		*f = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexID(n.String())
	return nil
}

func (f FlexID) MarshalJSON() ([]byte, error) {
	if numberRe.MatchString(string(f)) {
		return []byte(f), nil
	}
	return json.Marshal(string(f))
}

func (f FlexID) String() string { return string(f) }

// Matches reports whether a server with this id and uuid is the one the user
// named. uuid is empty when the API does not send it yet.
func Matches(ref string, id FlexID, uuid string) bool {
	if ref == "" {
		return false
	}
	return string(id) == ref || (uuid != "" && strings.EqualFold(uuid, ref))
}
