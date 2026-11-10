package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

const recordPhotoRefsGoldenPath = "frontend/src/lib/testdata/record_photo_refs.json"

// The names "record_photos_changed" gives transfers, for the UI's
// transferPhotoRef to match (recordPhotos.test.ts): a page that named a
// transfer differently would never redraw its changed photo.
func TestRecordPhotoRefsGolden(t *testing.T) {
	type refCase struct {
		ClientRef  string `json:"clientRef"`
		TransferID int64  `json:"transferID"`
		Ref        string `json:"ref"`
	}
	cases := []refCase{
		{ClientRef: "tr-6a1f", TransferID: 0},
		{ClientRef: "tr-6a1f", TransferID: 12}, // sent by this till: its own ref wins
		{ClientRef: "", TransferID: 5},
		{ClientRef: "", TransferID: 55},
		{ClientRef: "", TransferID: 1234567},
	}
	for i := range cases {
		cases[i].Ref = transferPhotoRef(cases[i].ClientRef, cases[i].TransferID)
	}
	got, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(recordPhotoRefsGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(recordPhotoRefsGoldenPath)
	if err != nil {
		t.Fatalf("%v (write it with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("transfer photo refs changed; update recordPhotos.ts to match and rewrite %s with KASHI_UPDATE_GOLDEN=1:\n%s", recordPhotoRefsGoldenPath, got)
	}
}
