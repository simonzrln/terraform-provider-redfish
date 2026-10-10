/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

// Package ami is a minimal client for the private REST API behind the AMI MegaRAC
// web UI (/api/...). It covers what Redfish does not expose on these BMCs, such as
// fan modes, fan curves and sensor thresholds.
package ami

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// Client is a logged-in web API session. Call Logout when done: the BMC only has a
// handful of session slots.
type Client struct {
	base string
	http *http.Client
	csrf string
}

// Login opens a web session. transport may be nil.
func Login(endpoint, user, password string, transport http.RoundTripper) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &Client{base: strings.TrimRight(endpoint, "/"), http: &http.Client{Transport: transport, Jar: jar}}
	form := url.Values{"username": {user}, "password": {password}}
	resp, err := c.http.PostForm(c.base+"/api/session", form)
	if err != nil {
		return nil, fmt.Errorf("ami login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ami login: HTTP %d", resp.StatusCode)
	}
	var s struct {
		CSRFToken string `json:"CSRFToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil || s.CSRFToken == "" {
		return nil, fmt.Errorf("ami login: no CSRF token in response")
	}
	c.csrf = s.CSRFToken
	return c, nil
}

// Logout ends the web session.
func (c *Client) Logout() {
	if resp, err := c.do(http.MethodDelete, "/api/session", nil); err == nil {
		resp.Body.Close()
	}
}

// Get decodes the JSON at path into out.
func (c *Client) Get(path string, out interface{}) error {
	return c.call(http.MethodGet, path, nil, out)
}

// Put sends in as JSON to path and decodes the reply into out (out may be nil).
func (c *Client) Put(path string, in, out interface{}) error {
	return c.call(http.MethodPut, path, in, out)
}

// Post is Put for singleton objects without an id, which the web UI saves via POST.
func (c *Client) Post(path string, in, out interface{}) error {
	return c.call(http.MethodPost, path, in, out)
}

func (c *Client) call(method, path string, in, out interface{}) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	resp, err := c.do(method, path, body)
	if err != nil {
		return fmt.Errorf("ami %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ami %s %s: HTTP %d: %s", method, path, resp.StatusCode, bytes.TrimSpace(raw))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("ami %s %s: decode: %w", method, path, err)
	}
	return nil
}

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-CSRFTOKEN", c.csrf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}
