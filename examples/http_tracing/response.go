package main

// Response represents the JSON response from the server
type Response struct {
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
	Data    string `json:"data"`
}
