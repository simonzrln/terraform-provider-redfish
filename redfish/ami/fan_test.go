/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package ami

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSetCurvePacksAndPads(t *testing.T) {
	row := Object{"id": 1.0, "Hysteresis": 2.0}
	pts := []CurvePoint{{30, 50}, {60, 55}, {95, 100}}
	if err := SetCurve(row, pts, 24, 1, 120, 20, 100); err != nil {
		t.Fatal(err)
	}
	if got := Curve(row, 24); !reflect.DeepEqual(got, []CurvePoint{{30, 50}, {60, 55}, {95, 100}}) {
		t.Fatalf("curve = %v", got)
	}
	if row["temp4"] != 255 || row["duty24"] != 255 || row["Hysteresis"] != 2.0 {
		t.Fatalf("padding or untouched fields wrong: %v", row)
	}
}

func TestSetCurveRejectsWhatTheBMCRejects(t *testing.T) {
	cases := map[string][]CurvePoint{
		"flat duty":      {{30, 50}, {40, 50}},
		"duplicate temp": {{30, 50}, {30, 60}},
		"duty too low":   {{30, 10}},
		"temp too high":  {{130, 50}},
		"empty":          nil,
		"unsorted":       {{60, 55}, {30, 50}},
	}
	for name, pts := range cases {
		if err := SetCurve(Object{}, pts, 24, 1, 120, 20, 100); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestCurveReadsLiveRow(t *testing.T) {
	// Table 1 as returned by the B650D4U on 2026-10-10 (trimmed to 12 slots).
	var row Object
	_ = json.Unmarshal([]byte(`{"id":1,"temp1":30,"temp2":40,"temp3":50,"temp4":60,"temp5":65,"temp6":70,"temp7":75,"temp8":80,"temp9":85,"temp10":90,"temp11":95,"temp12":255,
		"duty1":50,"duty2":51,"duty3":52,"duty4":55,"duty5":60,"duty6":65,"duty7":70,"duty8":75,"duty9":80,"duty10":90,"duty11":100,"duty12":255}`), &row)
	got := Curve(row, 12)
	if len(got) != 11 || got[0] != (CurvePoint{30, 50}) || got[10] != (CurvePoint{95, 100}) {
		t.Fatalf("curve = %v", got)
	}
}

func TestFanMask(t *testing.T) {
	// Live values: SelectedFan 65 = FAN1+FAN7 (CPU), 42 = FAN2+FAN4+FAN6 (case).
	if FanMask([]int{1, 7}) != 65 || FanMask([]int{2, 4, 6}) != 42 {
		t.Fatal("FanMask")
	}
	if !reflect.DeepEqual(MaskFans(42), []int{2, 4, 6}) {
		t.Fatal("MaskFans")
	}
}
