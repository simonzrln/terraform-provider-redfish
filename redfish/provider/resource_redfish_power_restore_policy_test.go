/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPowerRestorePolicy(t *testing.T) {
	p, servers, f := fakeRedfishProvider(t)
	ctx := context.Background()
	m := &models.PowerRestorePolicy{RedfishServer: servers, Policy: types.StringValue("LastState"), SystemID: types.StringUnknown()}
	if err := syncPowerRestorePolicy(ctx, p, m, true); err != nil {
		t.Fatal(err)
	}
	if b := f.patches[fakeSystem]; b["PowerRestorePolicy"] != "LastState" || len(b) != 1 {
		t.Fatalf("PATCH body = %v", b)
	}
	if m.Policy.ValueString() != "LastState" || m.ID.ValueString() != fakeSystem || m.SystemID.ValueString() != "Self" {
		t.Fatalf("state = %+v", m)
	}

	delete(f.patches, fakeSystem)
	if err := syncPowerRestorePolicy(ctx, p, m, true); err != nil || f.patches[fakeSystem] != nil {
		t.Fatalf("unchanged policy must not be written: err %v, body %v", err, f.patches[fakeSystem])
	}

	f.docs[fakeSystem]["PowerRestorePolicy"] = "AlwaysOn"
	if err := syncPowerRestorePolicy(ctx, p, m, false); err != nil || m.Policy.ValueString() != "AlwaysOn" {
		t.Fatalf("read must report drift: err %v, policy %v", err, m.Policy)
	}

	interval, timeout := powerRestorePollInterval, powerRestorePollTimeout
	powerRestorePollInterval, powerRestorePollTimeout = time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { powerRestorePollInterval, powerRestorePollTimeout = interval, timeout })
	f.frozen[fakeSystem] = true
	m.Policy = types.StringValue("AlwaysOff")
	if err := syncPowerRestorePolicy(ctx, p, m, true); err == nil || !strings.Contains(err.Error(), "did not store") {
		t.Fatalf("a policy the BMC ignores must fail, got %v", err)
	}
}
