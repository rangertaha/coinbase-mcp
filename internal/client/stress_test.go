// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
)

// TestClient_ConcurrentUse exercises the documented guarantee that a Client is
// safe for concurrent use. Run under -race, any shared-state mutation in
// Do/buildRequest surfaces here.
func TestClient_ConcurrentUse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}, WithHeader("X-Pin", "v1"), WithUserAgent("stress"))

	const goroutines = 16
	const requests = 25

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*requests)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < requests; i++ {
				var out struct {
					OK bool `json:"ok"`
				}
				if err := c.GetJSON(context.Background(), "/x", nil, &out); err != nil {
					errs <- err
					return
				}
				if !out.OK {
					errs <- io.ErrUnexpectedEOF
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent request failed: %v", err)
	}
}
