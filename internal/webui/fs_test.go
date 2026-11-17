package webui

import "testing"

func TestStubIsNotBuilt(t *testing.T) {
	if Built() {
		t.Fatal("Platzhalter-index.html darf nicht als gebaute UI gelten")
	}
	fsys, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	f, err := fsys.Open("index.html")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}
