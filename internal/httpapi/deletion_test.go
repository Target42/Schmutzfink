package httpapi

import (
	"testing"
	"time"

	"schmutzfink/internal/auth"
	"schmutzfink/internal/records"
)

func TestPresentPhotoIncludesDeletion(t *testing.T) {
	at := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	got := presentPhoto(records.Photo{
		ID:             "abc",
		UploadedBy:     "u1",
		UploadedByName: "anna",
		ContentType:    "image/jpeg",
		Deletion: &records.DeletionRequest{
			RequestedAt:     at,
			RequestedBy:     "u2",
			RequestedByName: "bert",
			Reason:          "doppelt",
		},
	})
	req, ok := got["deletion_request"].(map[string]any)
	if !ok {
		t.Fatal("deletion_request fehlt")
	}
	if req["reason"] != "doppelt" {
		t.Fatalf("reason: %v", req["reason"])
	}
	if req["requested_by_name"] != "bert" {
		t.Fatalf("requested_by_name: %v", req["requested_by_name"])
	}
}

func TestPresentPhotoWithoutDeletion(t *testing.T) {
	got := presentPhoto(records.Photo{ID: "abc", ContentType: "image/jpeg"})
	if got["deletion_request"] != nil {
		t.Fatalf("deletion_request: %v", got["deletion_request"])
	}
}

func TestDeletionForbidden(t *testing.T) {
	officer := auth.Principal{UserID: "a", Role: auth.RoleUser, CanDelete: true}
	if msg := deletionForbidden(officer, records.Photo{}); msg != "" {
		t.Fatalf("no request: %q", msg)
	}
	if msg := deletionForbidden(officer, records.Photo{Deletion: &records.DeletionRequest{RequestedBy: "b"}}); msg != "" {
		t.Fatalf("other request: %q", msg)
	}
	if msg := deletionForbidden(officer, records.Photo{Deletion: &records.DeletionRequest{RequestedBy: "a"}}); msg == "" {
		t.Fatal("own request must be blocked")
	}
	if msg := deletionForbidden(auth.Principal{Role: auth.RoleAdmin}, records.Photo{}); msg == "" {
		t.Fatal("admin without flag")
	}
	if msg := deletionForbidden(auth.Principal{Role: auth.RoleSearcher, CanDelete: true}, records.Photo{}); msg == "" {
		t.Fatal("searcher")
	}
}
