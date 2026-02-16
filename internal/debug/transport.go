package debug

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// debugTransport wraps an http.RoundTripper to log requests and responses
type DebugTransport struct {
	Transport http.RoundTripper
}

func (d *DebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("Making request to: %s %s\n", req.Method, req.URL.String())

	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		fmt.Printf("Request body: %s\n", string(bodyBytes))
	}

	resp, err := d.Transport.RoundTrip(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return resp, err
	}

	fmt.Printf("Response status: %d\n", resp.StatusCode)

	if resp.Body != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		fmt.Printf("Response body: %s\n", string(bodyBytes))
	}

	return resp, err
}
