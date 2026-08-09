package spec

import "testing"

func TestDeriveMaterializationState(t *testing.T) {
	record := MaterializationRecord{
		Artifact: MaterializationArtifact{Revision: 2, Hash: "artifact"},
		Source: MaterializationSource{
			Outline: 3,
			Spec:    4,
			Design:  5,
			Hash:    "source",
		},
	}
	tests := []struct {
		name      string
		hasHTML   bool
		record    *MaterializationRecord
		outline   int
		slideSpec int
		design    int
		artifact  string
		source    string
		want      string
	}{
		{name: "missing html", record: &record, want: "not_materialized"},
		{name: "missing record", hasHTML: true, outline: 3, slideSpec: 4, design: 5, want: "unknown"},
		{name: "outline stale", hasHTML: true, record: &record, outline: 4, slideSpec: 4, design: 5, artifact: "artifact", source: "changed", want: "spec_stale"},
		{name: "spec stale", hasHTML: true, record: &record, outline: 3, slideSpec: 5, design: 5, artifact: "artifact", source: "changed", want: "spec_stale"},
		{name: "design stale", hasHTML: true, record: &record, outline: 3, slideSpec: 4, design: 6, artifact: "artifact", source: "changed", want: "design_stale"},
		{name: "future source", hasHTML: true, record: &record, outline: 2, slideSpec: 4, design: 5, artifact: "artifact", source: "source", want: "unknown"},
		{name: "artifact drift", hasHTML: true, record: &record, outline: 3, slideSpec: 4, design: 5, artifact: "changed", source: "source", want: "unknown"},
		{name: "source drift", hasHTML: true, record: &record, outline: 3, slideSpec: 4, design: 5, artifact: "artifact", source: "changed", want: "unknown"},
		{name: "fresh", hasHTML: true, record: &record, outline: 3, slideSpec: 4, design: 5, artifact: "artifact", source: "source", want: "fresh"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := DeriveMaterializationState(
				test.hasHTML,
				test.record,
				test.outline,
				test.slideSpec,
				test.design,
				test.artifact,
				test.source,
			)
			if got != test.want {
				t.Fatalf("state=%q want=%q", got, test.want)
			}
		})
	}
}
