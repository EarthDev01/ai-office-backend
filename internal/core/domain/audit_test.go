package domain

import "testing"

func TestDiffFields_OnlyRealChanges(t *testing.T) {
	before := Office{ID: "o", Label: "old", AllowedOrigins: nil, Theme: "auto", Placement: Placement{Position: "bottom-right"}}
	after := before
	after.Label = "new"
	after.AllowedOrigins = []string{} // nil → [] ไม่นับว่าแก้
	after.Placement.OffsetX = 20

	got := DiffFields(before, after, "updated_at")
	if len(got) != 2 || got[0].Field != "label" || got[1].Field != "placement" {
		t.Fatalf("want label+placement, got %+v", got)
	}
	if got[0].Before != "old" || got[0].After != "new" {
		t.Fatalf("label values: %+v", got[0])
	}
}
