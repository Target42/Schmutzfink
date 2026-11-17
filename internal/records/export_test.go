package records

import "testing"

func TestApplyListLimit(t *testing.T) {
	f := ListFilter{}
	applyListLimit(&f)
	if f.Limit != 40 {
		t.Fatalf("default: %d", f.Limit)
	}

	f = ListFilter{Limit: 500}
	applyListLimit(&f)
	if f.Limit != 40 {
		t.Fatalf("over max without export: %d", f.Limit)
	}

	f = ListFilter{ForExport: true}
	applyListLimit(&f)
	if f.Limit != ExportMax {
		t.Fatalf("export default: %d", f.Limit)
	}

	f = ListFilter{ForExport: true, Limit: ExportMax + 1}
	applyListLimit(&f)
	if f.Limit != ExportMax {
		t.Fatalf("export over max: %d", f.Limit)
	}
}
