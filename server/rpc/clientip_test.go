package rpc

import (
	"encoding/json"
	"testing"
)

// decodeCall unpacks a message the way a reader would.
func decodeCall(t *testing.T, body []byte) (method string, args map[string]any) {
	t.Helper()

	var envelope struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("the message is not valid JSON any more: %v (%s)", err, body)
	}

	if len(envelope.Params) == 0 {
		return envelope.Method, nil
	}

	if err := json.Unmarshal(envelope.Params[0], &args); err != nil {
		t.Fatalf("the arguments are not an object any more: %v (%s)", err, body)
	}

	return envelope.Method, args
}

func TestWithClientIPAddsTheAddressToADownload(t *testing.T) {
	body := []byte(`{"method":"Service.Exec","params":[{"url":"https://example.com/v","params":["-f","best"]}],"id":1}`)

	method, args := decodeCall(t, withClientIP(body, "192.168.68.168"))

	if method != "Service.Exec" {
		t.Errorf("the method must survive the rewrite, got %q", method)
	}
	if args["from_ip"] != "192.168.68.168" {
		t.Errorf("want the address in the arguments, got %v", args["from_ip"])
	}
	if args["url"] != "https://example.com/v" {
		t.Errorf("the other arguments must survive, got %v", args["url"])
	}
	if params, ok := args["params"].([]any); !ok || len(params) != 2 {
		t.Errorf("the yt-dlp arguments must survive, got %v", args["params"])
	}
}

func TestWithClientIPLeavesOtherCallsAlone(t *testing.T) {
	// A call that does not start a download carries no address, and the
	// bytes are handed on untouched.
	for _, method := range []string{
		"Service.Running", "Service.Formats", "Service.KillDownload", "Service.Version",
	} {
		body := []byte(`{"method":"` + method + `","params":[{"url":"x"}],"id":1}`)

		got := withClientIP(body, "192.168.68.168")

		if string(got) != string(body) {
			t.Errorf("%s should not have been rewritten: %s", method, got)
		}
	}
}

func TestWithClientIPHandlesABatch(t *testing.T) {
	body := []byte(`[{"method":"Service.Running","params":[],"id":1},{"method":"Service.Exec","params":[{"url":"u"}],"id":2}]`)

	var batch []json.RawMessage
	if err := json.Unmarshal(withClientIP(body, "10.0.0.5"), &batch); err != nil {
		t.Fatalf("the batch is not a batch any more: %v", err)
	}

	if len(batch) != 2 {
		t.Fatalf("want two calls, got %d", len(batch))
	}

	// The first is untouched...
	if string(batch[0]) == "" {
		t.Fatal("the first call vanished")
	}
	if _, args := decodeCall(t, batch[0]); args["from_ip"] != nil {
		t.Errorf("Service.Running should not carry an address: %v", args)
	}

	// ...and the second has it.
	if _, args := decodeCall(t, batch[1]); args["from_ip"] != "10.0.0.5" {
		t.Errorf("want the address on the download, got %v", args["from_ip"])
	}
}

func TestWithClientIPWithoutAnAddressChangesNothing(t *testing.T) {
	// No address resolved means the field would be empty, and an empty
	// field is what the absent one already means.
	body := []byte(`{"method":"Service.Exec","params":[{"url":"u"}],"id":1}`)

	if got := withClientIP(body, ""); string(got) != string(body) {
		t.Errorf("an empty address should leave the message alone: %s", got)
	}
}

func TestWithClientIPIgnoresWhatItCannotParse(t *testing.T) {
	// A message the server cannot understand is still the JSON-RPC layer's
	// to reject, with its own error. Rewriting must not turn a bad request
	// into a different bad request.
	for _, body := range []string{
		`not json`,
		`{"method":"Service.Exec"}`,
		`{"method":"Service.Exec","params":"not-an-array"}`,
		`{"method":"Service.Exec","params":["not-an-object"]}`,
	} {
		if got := withClientIP([]byte(body), "1.2.3.4"); string(got) != body {
			t.Errorf("%q should have been left alone, got %q", body, got)
		}
	}
}
