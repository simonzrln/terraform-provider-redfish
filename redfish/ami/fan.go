/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package ami

import (
	"fmt"
	"strconv"
)

// Fan header modes as stored in /api/settings/fan/mode (fanN).
var FanModes = map[string]int{"default": 0, "customized": 1, "manual": 2}

// FanModeName is the inverse of FanModes.
func FanModeName(v int) string {
	for k, n := range FanModes {
		if n == v {
			return k
		}
	}
	return strconv.Itoa(v)
}

// unusedSlot marks an empty curve point or manual duty.
const unusedSlot = 255

// Object is a raw API object. The web API wants writes to echo every field it
// returned, so resources edit a copy of the GET reply instead of building structs.
type Object map[string]interface{}

// Int reads a numeric field (JSON numbers decode as float64).
func (o Object) Int(key string) int {
	switch v := o[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

// CurvePoint is one step of an open-loop fan table: from Temp °C on, run at Duty %.
type CurvePoint struct{ Temp, Duty int }

// Curve reads the points of an open-loop table row.
func Curve(row Object, entries int) []CurvePoint {
	var pts []CurvePoint
	for i := 1; i <= entries; i++ {
		t, d := row.Int(fmt.Sprintf("temp%d", i)), row.Int(fmt.Sprintf("duty%d", i))
		if t == unusedSlot && d == unusedSlot {
			continue
		}
		pts = append(pts, CurvePoint{t, d})
	}
	return pts
}

// SetCurve validates pts like the web UI does and writes them into row, packed into
// slots 1..n and padded with 255. Points must already be in ascending order, so the
// configuration reads back exactly as written.
func SetCurve(row Object, pts []CurvePoint, entries, minTemp, maxTemp, minDuty, maxDuty int) error {
	if len(pts) == 0 || len(pts) > entries {
		return fmt.Errorf("a curve needs 1..%d points, got %d", entries, len(pts))
	}
	prev := CurvePoint{}
	for _, p := range pts {
		if p.Temp < minTemp || p.Temp > maxTemp {
			return fmt.Errorf("temperature %d outside %d..%d", p.Temp, minTemp, maxTemp)
		}
		if p.Duty < minDuty || p.Duty > maxDuty {
			return fmt.Errorf("duty %d outside %d..%d", p.Duty, minDuty, maxDuty)
		}
		// The BMC requires both columns strictly increasing.
		if p.Temp <= prev.Temp || p.Duty <= prev.Duty {
			return fmt.Errorf("temperatures and duties must both be strictly increasing (%d°C/%d%% after %d°C/%d%%)", p.Temp, p.Duty, prev.Temp, prev.Duty)
		}
		prev = p
	}
	for i := 1; i <= entries; i++ {
		t, d := unusedSlot, unusedSlot
		if i <= len(pts) {
			t, d = pts[i-1].Temp, pts[i-1].Duty
		}
		row[fmt.Sprintf("temp%d", i)] = t
		row[fmt.Sprintf("duty%d", i)] = d
	}
	return nil
}

// FanMask turns 1-based fan header numbers into the SelectedFan bitmask.
func FanMask(fans []int) int {
	m := 0
	for _, f := range fans {
		m |= 1 << (f - 1)
	}
	return m
}

// MaskFans is the inverse of FanMask.
func MaskFans(mask int) []int {
	var fans []int
	for i := 0; i < 16; i++ {
		if mask&(1<<i) != 0 {
			fans = append(fans, i+1)
		}
	}
	return fans
}

// FindRow returns the row with the given id from a collection reply.
func FindRow(rows []Object, id int) (Object, error) {
	for _, r := range rows {
		if r.Int("id") == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("row %d not found", id)
}
