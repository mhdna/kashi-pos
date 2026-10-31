package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPickBranches(t *testing.T) {
	cn, hr := &branch{Code: "CN"}, &branch{Code: "HR"}
	all := []*branch{cn, hr}
	cases := map[string][]*branch{"all": all, "none": nil, "": nil, "2": {hr}, "cn, 2": {cn, hr}, "1,CN": {cn}}
	for in, want := range cases {
		got, err := pickBranches(all, in)
		if err != nil || len(got) != len(want) {
			t.Fatalf("%q: got %v, %v", in, got, err)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q: got %v", in, got)
			}
		}
	}
	if _, err := pickBranches(all, "XX"); err == nil {
		t.Fatal("unknown branch accepted")
	}
}

func TestTargetBranchesSendsOnlyChosenBranchOfThisFlavor(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /app_updates/registers":
			_ = json.NewEncoder(w).Encode(map[string]any{"registers": []map[string]any{
				{"id": 1, "branch_code": "CN", "code": "CN1", "is_active": true, "app_flavor": "standard"},
				{"id": 2, "branch_code": "CN", "code": "CN2", "is_active": true, "app_flavor": "compact"},
				{"id": 3, "branch_code": "HR", "code": "HR1", "is_active": true, "app_flavor": "standard"},
				{"id": 4, "branch_code": "CN", "code": "CN3", "is_active": false, "app_flavor": "standard"},
			}})
		case "PUT /app_updates/target":
			_ = json.NewDecoder(r.Body).Decode(&sent)
			_, _ = w.Write([]byte(`{"updated":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if err := targetBranches(srv.URL, "t", 9, "1.2.0", "standard", "cn", false); err != nil {
		t.Fatal(err)
	}
	ids, _ := sent["register_ids"].([]any)
	if len(ids) != 1 || ids[0].(float64) != 1 || sent["release_id"].(float64) != 9 {
		t.Fatalf("sent %v", sent)
	}
}
