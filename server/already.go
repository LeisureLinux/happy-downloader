package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RunningElsewhere reports whether this application is already listening, and
// which version it says it is.
//
// The check is deliberate rather than a probe of the port: any program can
// hold a port, and telling the user "already running" about somebody else's
// server would be both wrong and confusing. The version endpoint is this
// application's own, and the shape of its answer is not one another program
// produces.
//
// A port held by something that does not answer is not called ours: it may
// still be starting, or it may not speak HTTP at all, and neither is a reason
// to quietly do nothing.
func RunningElsewhere(port int) (version string, running bool) {
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/version", port))
	if err != nil {
		return "", false
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	var body struct {
		RPCVersion string `json:"rpcVersion"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.RPCVersion == "" {
		return "", false
	}

	return body.RPCVersion, true
}

// WaitForRunningElsewhere waits for another copy to go away, then reports
// whether one is still there.
//
// A copy that is being replaced — by a service restart, by an installer — lets
// go of its port shortly after it is asked to stop. Concluding immediately
// would leave the replacement believing the old one is the one to use, and a
// service that exits successfully while nothing is running is worse than one
// that reports a failure.
func WaitForRunningElsewhere(port int, wait time.Duration) (version string, running bool) {
	deadline := time.Now().Add(wait)

	for {
		if version, running := RunningElsewhere(port); running {
			if time.Now().After(deadline) {
				return version, true
			}

			time.Sleep(250 * time.Millisecond)
			continue
		}

		// Nothing answers now. If something did answer at the start, it has
		// finished going away.
		return "", false
	}
}
