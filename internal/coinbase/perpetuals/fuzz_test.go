// SPDX-License-Identifier: MIT

package perpetuals

import (
	"encoding/json"
	"testing"
)

func FuzzNumberUnmarshal(f *testing.F) {
	f.Add(`0.25`)
	f.Add(`"0.25"`)
	f.Add(`null`)
	f.Add(`{}`)
	f.Add(`[]`)
	f.Add(`"`)
	f.Add(``)
	f.Add(`-1e10`)
	f.Fuzz(func(t *testing.T, in string) {
		var n Number
		err := n.UnmarshalJSON([]byte(in)) // must never panic
		if err != nil {
			return
		}
		// Whatever was accepted must re-marshal as a valid JSON string.
		out, mErr := json.Marshal(n)
		if mErr != nil {
			t.Fatalf("marshal of accepted value %q failed: %v", n, mErr)
		}
		var back string
		if uErr := json.Unmarshal(out, &back); uErr != nil {
			t.Fatalf("round-trip of %q failed: %v", n, uErr)
		}
	})
}
