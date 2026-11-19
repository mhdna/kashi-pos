package license

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Same seed as kashi-license/lease/lease_test.go.
var goldenSeed = []byte("kashi-license-golden-test-seed!!")

func TestGoldenLeaseFromServerVerifies(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		PublicKeyPEM string `json:"publicKeyPem"`
		Lease        Lease  `json:"lease"`
		Signed       Signed `json:"signed"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	pub, err := ParsePublicKey(g.PublicKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Equal(ed25519.NewKeyFromSeed(goldenSeed).Public()) {
		t.Fatal("golden key is not the shared test seed")
	}
	l, err := Verify(pub, g.Signed)
	if err != nil {
		t.Fatalf("server's golden lease does not verify here: %v (copy kashi-license/lease/testdata/golden.json)", err)
	}
	if l != g.Lease || l.Status != StatusOn || l.CheckEverySeconds != 60 {
		t.Fatalf("decoded %+v, want %+v", l, g.Lease)
	}
}

func TestShippedPublicKeyParses(t *testing.T) {
	if _, err := ParsePublicKey(ServerPublicKeyPEM); err != nil {
		t.Fatal(err)
	}
}

type signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

func newSigner() signer {
	priv := ed25519.NewKeyFromSeed(goldenSeed)
	return signer{priv, priv.Public().(ed25519.PublicKey)}
}

var t0 = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

func (s signer) state(status string, issued time.Time, days int, fp string) State {
	l := Lease{V: Version, InstallID: "inst-1", Fingerprint: fp, Status: status, LicensedTo: "Shop", IssuedAt: issued, ValidUntil: issued.Add(time.Duration(days) * 24 * time.Hour), CheckEverySeconds: 60}
	signed, _ := Sign(s.priv, l)
	return State{InstallID: "inst-1", InstallSecret: "sec", DBKey: hex64, Lease: signed}
}

const hex64 = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func TestJudge(t *testing.T) {
	s := newSigner()
	on := s.state(StatusOn, t0, 4, "fp")
	cases := []struct {
		name   string
		st     State
		fp     string
		now    time.Time
		ok     bool
		reason string
		warn   bool
	}{
		{"fresh", on, "fp", t0.Add(time.Minute), true, "", false},
		{"day 2 offline", on, "fp", t0.Add(47 * time.Hour), true, "", false},
		{"day 3 offline warns", on, "fp", t0.Add(49 * time.Hour), true, "", true},
		{"last second", on, "fp", t0.Add(96*time.Hour - time.Second), true, "", true},
		{"4 days offline", on, "fp", t0.Add(96 * time.Hour), false, ReasonExpired, false},
		{"a month offline", on, "fp", t0.Add(30 * 24 * time.Hour), false, ReasonExpired, false},
		{"other machine", on, "fp-other", t0, false, ReasonWrongMachine, false},
		{"turned off", s.state(StatusOff, t0, 0, "fp"), "fp", t0, false, ReasonOff, false},
		{"clock before issue", on, "fp", t0.Add(-time.Hour), false, ReasonClock, false},
		{"clock a little behind is fine", on, "fp", t0.Add(-10 * time.Minute), true, "", false},
		{"clock before last seen", func() State { x := on; x.LastSeen = t0.Add(60 * time.Hour); return x }(), "fp", t0.Add(30 * time.Hour), false, ReasonClock, false},
		{"tampered", func() State { x := on; x.Lease.Signature = x.Lease.Signature[:8] + "AAAAAAAA" + x.Lease.Signature[16:]; return x }(), "fp", t0, false, ReasonBadLease, false},
		{"lease of another install", func() State { x := on; x.InstallID = "inst-2"; return x }(), "fp", t0, false, ReasonWrongMachine, false},
		{"empty", State{}, "fp", t0, false, ReasonBadLease, false},
	}
	for _, c := range cases {
		d := Judge(s.pub, c.st, c.fp, c.now)
		if d.OK != c.ok || d.Reason != c.reason || d.Warn != c.warn {
			t.Errorf("%s: got ok=%v reason=%q warn=%v, want ok=%v reason=%q warn=%v", c.name, d.OK, d.Reason, d.Warn, c.ok, c.reason, c.warn)
		}
	}
	// A lease signed by any other key never lets a till run.
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	forged := signer{otherPriv, otherPriv.Public().(ed25519.PublicKey)}.state(StatusOn, t0, 3650, "fp")
	if d := Judge(s.pub, forged, "fp", t0); d.OK || d.Reason != ReasonBadLease {
		t.Fatalf("forged lease: %+v", d)
	}
}

// Over many random leases, clocks and last-seen times, Judge agrees with the
// rules stated plainly.
func TestJudgeRandomized(t *testing.T) {
	s := newSigner()
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 2000; i++ {
		days := 1 + rng.Intn(14)
		status := StatusOn
		if rng.Intn(5) == 0 {
			status = StatusOff
		}
		st := s.state(status, t0, days, "fp")
		if rng.Intn(2) == 0 {
			st.LastSeen = t0.Add(time.Duration(rng.Intn(20*24)) * time.Hour)
		}
		now := t0.Add(time.Duration(rng.Intn(40*24*60)-5*24*60) * time.Minute)
		d := Judge(s.pub, st, "fp", now)
		valid := t0.Add(time.Duration(days) * 24 * time.Hour)
		clockOK := !now.Before(t0.Add(-ClockSlack)) && (st.LastSeen.IsZero() || !now.Before(st.LastSeen.Add(-ClockSlack)))
		want := status == StatusOn && clockOK && now.Before(valid)
		if d.OK != want {
			t.Fatalf("case %d: status=%s days=%d now=%v lastSeen=%v: ok=%v reason=%q, want %v", i, status, days, now, st.LastSeen, d.OK, d.Reason, want)
		}
		if d.OK && (d.Left != valid.Sub(now) || d.Warn != (valid.Sub(now) < WarnBefore)) {
			t.Fatalf("case %d: left=%v warn=%v", i, d.Left, d.Warn)
		}
	}
}

func TestStateRoundTripAndMachineBinding(t *testing.T) {
	s := newSigner()
	path := StatePath(t.TempDir())
	if _, err := LoadState(path, "fp"); err != ErrNoState {
		t.Fatalf("missing file = %v", err)
	}
	st := s.state(StatusOn, t0, 4, "fp")
	st.LastSeen = t0
	if err := SaveState(path, "fp", st); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(path, "fp")
	if err != nil || got.InstallSecret != st.InstallSecret || got.DBKey != st.DBKey || !got.LastSeen.Equal(t0) || got.Lease != st.Lease {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
	raw, _ := os.ReadFile(path)
	for _, secret := range []string{st.InstallSecret, hex64, "inst-1"} {
		if len(secret) > 2 && json.Valid(raw) || containsBytes(raw, secret) {
			t.Fatalf("license file shows %q in the clear", secret)
		}
	}
	// Copied to a PC with other hardware: it doesn't open.
	if _, err := LoadState(path, "other-fp"); err != ErrUnseal {
		t.Fatalf("other machine = %v", err)
	}
	// One byte changed: it doesn't open.
	raw[len(raw)-1] ^= 1
	_ = os.WriteFile(path, raw, 0o600)
	if _, err := LoadState(path, "fp"); err != ErrUnseal {
		t.Fatalf("tampered = %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
}

func containsBytes(b []byte, s string) bool {
	return len(s) > 0 && bytesIndex(b, []byte(s)) >= 0
}

func bytesIndex(b, sub []byte) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		if string(b[i:i+len(sub)]) == string(sub) {
			return i
		}
	}
	return -1
}

func TestClientMapsAnswers(t *testing.T) {
	s := newSigner()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch {
		case r.URL.Path == "/v1/activate" && req["productKey"] == "GOOD":
			st := s.state(StatusOn, t0, 4, req["fingerprint"])
			_ = json.NewEncoder(w).Encode(ActivateResponse{InstallID: "inst-1", InstallSecret: "sec", DBKey: hex64, Lease: st.Lease})
		case r.URL.Path == "/v1/activate":
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"invalid_key"}`))
		case r.URL.Path == "/v1/checkin" && req["installSecret"] == "sec":
			_ = json.NewEncoder(w).Encode(map[string]any{"lease": s.state(StatusOff, t0, 0, "fp").Lease})
		case r.URL.Path == "/v1/checkin" && req["installSecret"] == "proxy":
			w.WriteHeader(502)
			_, _ = w.Write([]byte(`<html>Bad gateway</html>`))
		default:
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"unknown_install"}`))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL + "/")
	ctx := context.Background()
	got, err := c.Activate(ctx, ActivateRequest{ProductKey: "GOOD", Fingerprint: "fp"})
	if err != nil || got.DBKey != hex64 {
		t.Fatalf("activate = %+v %v", got, err)
	}
	if _, err := Verify(s.pub, got.Lease); err != nil {
		t.Fatal(err)
	}
	_, err = c.Activate(ctx, ActivateRequest{ProductKey: "BAD"})
	if ae, ok := err.(*APIError); !ok || ae.Code != CodeInvalidKey || ae.Status != 404 {
		t.Fatalf("bad key = %v", err)
	}
	signed, err := c.CheckIn(ctx, CheckInRequest{InstallSecret: "sec"})
	if l, verr := Verify(s.pub, signed); err != nil || verr != nil || l.Status != StatusOff {
		t.Fatalf("checkin = %v %v", err, verr)
	}
	_, err = c.CheckIn(ctx, CheckInRequest{InstallSecret: "proxy"})
	if _, ok := err.(*APIError); ok || err == nil {
		t.Fatalf("a 502 page must count as not reaching the server, got %v", err)
	}
	_, err = c.CheckIn(ctx, CheckInRequest{InstallSecret: "x"})
	if ae, ok := err.(*APIError); !ok || ae.Code != CodeUnknownInstall {
		t.Fatalf("unknown = %v", err)
	}
	dead := NewClient("http://127.0.0.1:1")
	if _, err := dead.CheckIn(ctx, CheckInRequest{}); err == nil {
		t.Fatal("no server, no error")
	} else if _, ok := err.(*APIError); ok {
		t.Fatal("unreachable server reported as an answer")
	}
}

func TestStatePathIsMachineLevel(t *testing.T) {
	if got := StatePath("/cfg/kashi-pos"); got != filepath.Join("/cfg/kashi-pos", "license.dat") {
		t.Fatal(got)
	}
}
