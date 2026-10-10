/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-redfish/redfish/ami"
	"terraform-provider-redfish/redfish/models"

	"github.com/stmcginnis/gofish"
	redfishcommon "github.com/stmcginnis/gofish/common"
)

// withRedfish runs fn with a Redfish session, holding the per-endpoint lock.
func withRedfish(p *redfishProvider, servers *[]models.RedfishServer, fn func(*gofish.Service) error) error {
	c, err := resolveServer(p, servers)
	if err != nil {
		return err
	}
	redfishMutexKV.Lock(c.endpoint)
	defer redfishMutexKV.Unlock(c.endpoint)
	api, err := NewConfig(p, servers)
	if err != nil {
		return err
	}
	defer api.Logout()
	return fn(api.Service)
}

// withAMI runs fn with a session on the AMI MegaRAC web API, holding the same
// per-endpoint lock: the BMC has few session slots and serialises writes anyway.
func withAMI(p *redfishProvider, servers *[]models.RedfishServer, fn func(*ami.Client) error) error {
	c, err := resolveServer(p, servers)
	if err != nil {
		return err
	}
	redfishMutexKV.Lock(c.endpoint)
	defer redfishMutexKV.Unlock(c.endpoint)
	client, err := ami.Login(c.endpoint, c.user, c.password, p.GetHTTPClientWithTLS(c.insecure).Transport)
	if err != nil {
		return err
	}
	defer client.Logout()
	return fn(client)
}

// patchWithETag PATCHes uri with If-Match set to the resource's current ETag
// (AMI MegaRAC rejects PATCH without it on several resources).
func patchWithETag(c redfishcommon.Client, uri string, payload interface{}) error {
	var head struct {
		ETag string `json:"@odata.etag"`
	}
	if err := getJSON(c, uri, &head); err != nil {
		return err
	}
	var headers map[string]string
	if head.ETag != "" {
		headers = map[string]string{"If-Match": head.ETag}
	}
	resp, err := c.PatchWithHeaders(uri, payload, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PATCH %s: HTTP %d: %s", uri, resp.StatusCode, b)
	}
	// Redfish reports partial failures as 200 with @Message.ExtendedInfo; surface them.
	var body map[string]json.RawMessage
	if b, _ := io.ReadAll(resp.Body); json.Unmarshal(b, &body) == nil {
		if info, ok := body["@Message.ExtendedInfo"]; ok && len(info) > 2 {
			if msg := string(info); containsError(msg) {
				return fmt.Errorf("PATCH %s: %s", uri, msg)
			}
		}
	}
	return nil
}

func containsError(extendedInfo string) bool {
	var infos []struct {
		MessageSeverity string
		Severity        string
	}
	if json.Unmarshal([]byte(extendedInfo), &infos) != nil {
		return false
	}
	for _, i := range infos {
		if i.MessageSeverity == "Critical" || i.MessageSeverity == "Warning" || i.Severity == "Critical" || i.Severity == "Warning" {
			return true
		}
	}
	return false
}
