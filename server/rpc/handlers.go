package rpc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	middlewares "github.com/LeisureLinux/happy-downloader/v4/server/middleware"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSockets JSON-RPC handler
func WebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer c.Close()

	// notify client that conn is open and ok
	c.WriteJSON(struct{ Status string }{Status: "connected"})

	ip := middlewares.ClientIP(r)

	for {
		mtype, reader, err := c.NextReader()
		if err != nil {
			break
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			break
		}

		res := newRequest(bytes.NewReader(withClientIP(body, ip))).Call()

		writer, err := c.NextWriter(mtype)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			break
		}

		io.Copy(writer, res)
	}
}

// HTTP-POST JSON-RPC handler
func Post(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res := newRequest(bytes.NewReader(withClientIP(body, middlewares.ClientIP(r)))).Call()

	if _, err := io.Copy(w, res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// startsDownload reports whether a method creates a download.
//
// Only those carry the client address. Putting it in every call would be
// bytes on the wire for a field nothing reads, and it would mean rewriting
// messages the server does not care about.
func startsDownload(method string) bool {
	switch method {
	case "Service.Exec", "Service.ExecPlaylist", "Service.ExecLivestream":
		return true
	}
	return false
}

// withClientIP puts the caller's address into a download request.
//
// The address is known only to the handler that received the call — the
// JSON-RPC layer decodes the arguments itself, and the download outlives the
// request by hours — so it is written into the message before it is decoded.
// It is taken from the socket or the forwarded headers, never from the
// browser: a client that could name itself could name any machine on the LAN.
func withClientIP(body []byte, ip string) []byte {
	if ip == "" {
		return body
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body
	}

	// A batch is an array of calls; anything else is a single one.
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			return body
		}

		changed := false
		for i, call := range batch {
			if next, ok := injectClientIP(call, ip); ok {
				batch[i] = next
				changed = true
			}
		}

		if !changed {
			return body
		}

		out, err := json.Marshal(batch)
		if err != nil {
			return body
		}
		return out
	}

	if next, ok := injectClientIP(trimmed, ip); ok {
		return next
	}

	return body
}

// injectClientIP adds the address to one call, if it is one that starts a
// download. It reports whether the message changed.
func injectClientIP(raw json.RawMessage, ip string) (json.RawMessage, bool) {
	var call struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}

	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, false
	}

	if !startsDownload(call.Method) || len(call.Params) == 0 {
		return nil, false
	}

	var args map[string]any
	if err := json.Unmarshal(call.Params[0], &args); err != nil {
		return nil, false
	}

	args["from_ip"] = ip

	encodedArgs, err := json.Marshal(args)
	if err != nil {
		return nil, false
	}

	params := append([]json.RawMessage{encodedArgs}, call.Params[1:]...)

	encodedParams, err := json.Marshal(params)
	if err != nil {
		return nil, false
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, false
	}

	envelope["params"] = encodedParams

	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, false
	}

	return out, true
}
