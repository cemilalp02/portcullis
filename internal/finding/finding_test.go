package finding

import (
	"encoding/json"
	"testing"
)

func TestParseSeverity(t *testing.T) {
	for i, name := range []string{"info", "LOW", "Medium", "high", "critical"} {
		got, err := ParseSeverity(name)
		if err != nil || got != Severity(i) {
			t.Errorf("ParseSeverity(%q) = %v, %v; want %v", name, got, err, Severity(i))
		}
	}
	if _, err := ParseSeverity("severe"); err == nil {
		t.Error("ParseSeverity(severe) succeeded, want error")
	}
}

func TestSeverityAndConfidenceText(t *testing.T) {
	type pair struct {
		S Severity
		C Confidence
	}
	b, err := json.Marshal(pair{S: High, C: ConfidenceMedium})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"S":"high","C":"medium"}` {
		t.Errorf("marshal = %s", b)
	}
	var back pair
	if err := json.Unmarshal(b, &back); err != nil || back.S != High || back.C != ConfidenceMedium {
		t.Errorf("unmarshal = %+v, %v", back, err)
	}
	if err := json.Unmarshal([]byte(`{"S":"nope"}`), &back); err == nil {
		t.Error("unmarshal of unknown severity succeeded")
	}
	if err := json.Unmarshal([]byte(`{"C":"nope"}`), &back); err == nil {
		t.Error("unmarshal of unknown confidence succeeded")
	}
	if _, err := Severity(9).MarshalText(); err == nil {
		t.Error("marshal of invalid severity succeeded")
	}
	if _, err := Confidence(9).MarshalText(); err == nil {
		t.Error("marshal of invalid confidence succeeded")
	}
	if Severity(9).String() != "severity(9)" || Confidence(-1).String() != "confidence(-1)" {
		t.Error("String of invalid values")
	}
}

func TestSort(t *testing.T) {
	fs := []Finding{
		{RuleID: "B", Severity: Low, Path: "a", Line: 1},
		{RuleID: "A", Severity: Critical, Path: "z", Line: 9},
		{RuleID: "C", Severity: Low, Path: "a", Line: 1, Column: 2},
		{RuleID: "A", Severity: Low, Path: "a", Line: 1},
	}
	Sort(fs)
	var got []string
	for _, f := range fs {
		got = append(got, f.RuleID+f.Path)
	}
	want := []string{"Az", "Aa", "Ba", "Ca"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
