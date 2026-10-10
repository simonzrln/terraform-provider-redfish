/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package provider

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fakeObj = map[string]interface{}

func fakeRef(uri string) fakeObj { return fakeObj{"@odata.id": uri} }

const (
	fakeToken     = "tok"
	fakeSystem    = "/redfish/v1/Systems/Self"
	fakeBios      = fakeSystem + "/Bios"
	fakeBiosSD    = fakeBios + "/SD"
	fakeRegistry  = "BiosAttributeRegistryA2982.22.10.0"
	fakeRegistryJ = "/redfish/v1/Registries/BiosAttributeRegistryA2982.en-US.22.10.0.json"
)

// fakeRedfish emulates the Redfish resources of an AMI MegaRAC BMC that the BIOS and
// power restore resources use, with shapes and values captured from a B650D4U
// (firmware 6.09) on 2026-10-10: session login, ETag/If-Match, Bios with its SD
// settings object, and the BIOS attribute registry (trimmed, testdata/bios_registry.json).
type fakeRedfish struct {
	mu       sync.Mutex
	docs     map[string]fakeObj
	frozen   map[string]bool    // PATCH accepted (200) but not stored
	requests []string           // "METHOD path" of every request
	patches  map[string]fakeObj // last PATCH body per path
	gzipped  int                // responses sent gzip-compressed
	etag     int
}

func newFakeRedfish(t *testing.T) *fakeRedfish {
	t.Helper()
	b, err := os.ReadFile("testdata/bios_registry.json")
	if err != nil {
		t.Fatal(err)
	}
	var registry fakeObj
	if err := json.Unmarshal(b, &registry); err != nil {
		t.Fatal(err)
	}
	running := fakeObj{
		"CbsCpuSvmEnableRPL": "CbsCpuSvmEnableRPLEnabled", "CbsCmnGnbNbIOMMURPL": "CbsCmnGnbNbIOMMURPLEnabled",
		"CbsCmnGnbPowerSupplyIdleCtrlRPL": "CbsCmnGnbPowerSupplyIdleCtrlRPLTypicalCurrentIdle",
		"Pbs_CoreCountControl":            "Pbs_CoreCountControlDisabled", "Pbs_ProcessorAggregatorDevice": "Pbs_ProcessorAggregatorDeviceDisabled",
		"ACPLBMC000": "ACPLBMC000Nochange", "SECB001": "SECB001Disabled", "TER001": "Disabled", "TER002": "Disabled",
		"TER012": "TER012VT100Plus", "TER013": "TER013VT100Plus", "SETUP003": 1.0,
	}
	pending := maps.Clone(running)
	pending["ACPLBMC000"] = "ACPLBMC000PowerOn" // pending on the real board at capture time
	return &fakeRedfish{
		frozen:  map[string]bool{},
		patches: map[string]fakeObj{},
		docs: map[string]fakeObj{
			"/redfish/v1": {
				"@odata.id": "/redfish/v1", "Id": "RootService", "RedfishVersion": "1.15.1",
				"SessionService": fakeRef("/redfish/v1/SessionService"), "Links": fakeObj{"Sessions": fakeRef("/redfish/v1/SessionService/Sessions")},
				"Systems": fakeRef("/redfish/v1/Systems"), "Registries": fakeRef("/redfish/v1/Registries"),
			},
			"/redfish/v1/Systems": {"@odata.id": "/redfish/v1/Systems", "Members": []interface{}{fakeRef(fakeSystem)}, "Members@odata.count": 1},
			fakeSystem: {
				"@odata.id": fakeSystem, "@odata.etag": `"1791654002"`, "Id": "Self",
				"PowerRestorePolicy": "AlwaysOn", "Bios": fakeRef(fakeBios),
			},
			fakeBios: {
				"@odata.id": fakeBios, "@odata.etag": `"1791653999"`, "Id": "Bios", "AttributeRegistry": fakeRegistry,
				"@Redfish.Settings": fakeObj{"SettingsObject": fakeRef(fakeBiosSD)}, "Attributes": running,
			},
			fakeBiosSD:               {"@odata.id": fakeBiosSD, "@odata.etag": `"1791653999"`, "Id": "SD", "Attributes": pending},
			"/redfish/v1/Registries": {"@odata.id": "/redfish/v1/Registries", "Members": []interface{}{fakeRef("/redfish/v1/Registries/" + fakeRegistry)}},
			"/redfish/v1/Registries/" + fakeRegistry: {
				"@odata.id": "/redfish/v1/Registries/" + fakeRegistry, "Id": fakeRegistry, "Languages": []interface{}{"en-US"},
				"Location": []interface{}{fakeObj{"Language": "en-US", "Uri": fakeRegistryJ}},
			},
			fakeRegistryJ: registry,
		},
	}
}

func (f *fakeRedfish) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r.URL.Path = strings.TrimSuffix(r.URL.Path, "/") // gofish asks for /redfish/v1/
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	switch {
	case r.URL.Path == "/redfish/v1/SessionService/Sessions" && r.Method == http.MethodPost:
		w.Header().Set("X-Auth-Token", fakeToken)
		w.Header().Set("Location", "/redfish/v1/SessionService/Sessions/1")
		w.WriteHeader(http.StatusCreated)
		return
	case r.URL.Path != "/redfish/v1" && r.Header.Get("X-Auth-Token") != fakeToken:
		http.Error(w, "no session", http.StatusUnauthorized)
		return
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
		return
	}
	doc, ok := f.docs[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPatch && !f.patch(w, r, doc) {
		return
	}
	var out io.Writer = w
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		out = gz
		f.gzipped++
	}
	_ = json.NewEncoder(out).Encode(doc)
}

// patch merges the body into doc (one level deep, like Attributes) after checking If-Match.
func (f *fakeRedfish) patch(w http.ResponseWriter, r *http.Request, doc fakeObj) bool {
	if etag, _ := doc["@odata.etag"].(string); r.Header.Get("If-Match") != etag {
		http.Error(w, "precondition failed", http.StatusPreconditionFailed)
		return false
	}
	var body fakeObj
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	f.patches[r.URL.Path] = body
	if f.frozen[r.URL.Path] {
		return true
	}
	for k, v := range body {
		if sub, ok := v.(fakeObj); ok {
			maps.Copy(doc[k].(fakeObj), sub)
			continue
		}
		doc[k] = v
	}
	f.etag++
	doc["@odata.etag"] = fmt.Sprintf(`"%d"`, f.etag)
	return true
}

func (f *fakeRedfish) requested(method, path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if r == method+" "+path {
			return true
		}
	}
	return false
}

func fakeRedfishProvider(t *testing.T) (*redfishProvider, []models.RedfishServer, *fakeRedfish) {
	t.Helper()
	f := newFakeRedfish(t)
	srv := httptest.NewTLSServer(f)
	t.Cleanup(srv.Close)
	p := &redfishProvider{RetryConfig: DefaultRetryConfig()}
	servers := []models.RedfishServer{{
		Endpoint: types.StringValue(srv.URL), User: types.StringValue("admin"),
		Password: types.StringValue("pw"), SslInsecure: types.BoolValue(true),
	}}
	return p, servers, f
}
