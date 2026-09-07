// Package mcp is a Model Context Protocol server over stdio.
//
// JSON-RPC 2.0 with a handful of methods, which is all MCP is on the wire. It
// is implemented here rather than taken as a dependency because the protocol
// surface this server needs is about a hundred lines, and a dependency that
// large would be carrying a framework to avoid writing a switch statement.
//
// # WHY AN MCP SERVER AT ALL, WHEN THERE IS ALREADY A SKILL
//
// A skill teaches an agent to run the CLI. That works, and it means every
// answer arrives as text the agent has to parse back out of a terminal.
//
// Tools are different: the agent gets a typed result it did not have to scrape,
// and an argument list it cannot get subtly wrong. `logs --gaps` returning a
// list of intervals beats the same information as a block of aligned text that
// an agent has to re-read.
//
// The gates are unchanged and unchangeable from here. A tool call publishes a
// request; the station still decides whether to run it, still refuses an
// undeclared step, and still refuses an action without --allow-actions. An MCP
// client asks. It does not get to answer.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Version of the MCP protocol this speaks.
const protocolVersion = "2024-11-05"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool is one callable thing.
type Tool struct {
	Name string
	// Description is read by a model deciding whether to call this, so it says
	// what the tool does AND when not to reach for it. A description that only
	// names the happy path gets called in situations it cannot serve.
	Description string
	Schema      map[string]any
	Call        func(args map[string]any) (string, error)
}

// Server serves tools over stdio.
type Server struct {
	name    string
	version string
	tools   []Tool
	mu      sync.Mutex
	out     *json.Encoder
}

func NewServer(name, version string, tools []Tool) *Server {
	sort.Slice(tools, func(a, b int) bool { return tools[a].Name < tools[b].Name })
	return &Server{name: name, version: version, tools: tools}
}

// Serve reads requests until the input closes.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	s.out = json.NewEncoder(out)
	sc := bufio.NewScanner(in)
	// A tool result can carry a whole captured log, and the default 64KB limit
	// would truncate one silently. Truncating a log is the one thing this
	// product refuses to do anywhere else.
	sc.Buffer(make([]byte, 0, 256*1024), 32*1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.send(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		s.handle(req)
	}
	return sc.Err()
}

func (s *Server) send(r response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.JSONRPC = "2.0"
	_ = s.out.Encode(r)
}

func (s *Server) handle(req request) {
	// A notification has no id and gets no reply. Answering one is a protocol
	// error that some clients tolerate and others hang on.
	notify := len(req.ID) == 0

	switch req.Method {
	case "initialize":
		s.send(response{ID: req.ID, Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.name, "version": s.version},
		}})

	case "notifications/initialized", "notifications/cancelled":
		// Nothing to do, and nothing to answer.

	case "tools/list":
		list := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			list = append(list, map[string]any{
				"name": t.Name, "description": t.Description, "inputSchema": t.Schema,
			})
		}
		s.send(response{ID: req.ID, Result: map[string]any{"tools": list}})

	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			s.send(response{ID: req.ID, Error: &rpcError{Code: -32602, Message: "bad params"}})
			return
		}
		for _, t := range s.tools {
			if t.Name != p.Name {
				continue
			}
			text, err := t.Call(p.Arguments)
			if err != nil {
				// An error is returned as a RESULT with isError, not as a
				// JSON-RPC error. The distinction matters: a JSON-RPC error
				// means the call was malformed, and a model that sees one
				// tends to stop rather than read the message and adjust.
				s.send(response{ID: req.ID, Result: map[string]any{
					"content": []map[string]any{{"type": "text", "text": err.Error()}},
					"isError": true,
				}})
				return
			}
			s.send(response{ID: req.ID, Result: map[string]any{
				"content": []map[string]any{{"type": "text", "text": text}},
			}})
			return
		}
		s.send(response{ID: req.ID, Error: &rpcError{
			Code: -32602, Message: fmt.Sprintf("no tool named %q", p.Name)}})

	default:
		if notify {
			return
		}
		s.send(response{ID: req.ID, Error: &rpcError{
			Code: -32601, Message: "method not found: " + req.Method}})
	}
}

// Str reads a string argument.
func Str(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// Bool reads a boolean argument.
func Bool(args map[string]any, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

// StrMap reads a map of string to string, for the env of a run.
func StrMap(args map[string]any, key string) map[string]string {
	out := map[string]string{}
	m, ok := args[key].(map[string]any)
	if !ok {
		return out
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
