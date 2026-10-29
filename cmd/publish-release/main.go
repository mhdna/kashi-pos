// publish-release signs a GB Cloud build and uploads it to kashi as a remote
// update (see updater.go and kashi's api/app_update.go). Run it through the
// Makefile:
//
//	make update-keygen                      # once: create the release key
//	git tag v1.2.3 && make publish NOTES="…" # every release; asks which branches get it
//	make publish NOTES="…" BRANCHES=CN,HR    # …straight to those branches' tills
//	git tag v1.2.3 && make publish-all NOTES="…" # …and send it to every till
//
// Once every till it was sent to runs it, kashi deletes the build's file
// (unless App Updates' "Keep old builds" says otherwise).
//
// publish needs KASHI_URL (the same central URL the tills use, e.g.
// https://cloud.example.com/api) and KASHI_USER, an admin; KASHI_PASSWORD is
// asked for when it isn't set.
package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kashi-pos/internal/updatekey"

	"golang.org/x/term"
)

const updateKeyFile = "internal/updatekey/updatekey.go"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: publish-release keygen | publish -file F -version V -flavor standard|compact [-notes N] [-branches all|none|CODE,CODE | -target-all] [-force]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "publish":
		err = publish(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func defaultKeyPath() string {
	if p := os.Getenv("KASHI_CLOUD_UPDATE_KEY"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gb-cloud", "update-signing.key")
}

// keygen creates the release key pair. The private half stays on this
// machine; the public half goes into internal/updatekey, which every build
// after this one carries.
func keygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	keyPath := fs.String("key", defaultKeyPath(), "where to write the private key")
	_ = fs.Parse(args)

	if _, err := os.Stat(*keyPath); err == nil {
		return fmt.Errorf("%s already exists; refusing to replace it (tills only accept updates signed with the key they were built with)", *keyPath)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*keyPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(*keyPath, []byte(base64.StdEncoding.EncodeToString(priv)+"\n"), 0o600); err != nil {
		return err
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	src, err := os.ReadFile(updateKeyFile)
	if err != nil {
		return fmt.Errorf("run this from the kashi-pos repo root: %w", err)
	}
	re := regexp.MustCompile(`const PublicKey = ".*"`)
	if !re.Match(src) {
		return fmt.Errorf("%s has no PublicKey constant", updateKeyFile)
	}
	if err := os.WriteFile(updateKeyFile, re.ReplaceAll(src, []byte(fmt.Sprintf("const PublicKey = %q", pubB64))), 0o644); err != nil {
		return err
	}
	fmt.Printf("Release key written to %s\n", *keyPath)
	fmt.Printf("Public key %s written to %s -- commit it.\n", pubB64, updateKeyFile)
	fmt.Println("BACK UP THE PRIVATE KEY: without it, tills won't accept any new update.")
	return nil
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read release key (create it once with `make update-keygen`): %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%s is not a release key", path)
	}
	return ed25519.PrivateKey(key), nil
}

func publish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	file := fs.String("file", "", "the built GB Cloud .exe")
	version := fs.String("version", "", "its version, e.g. 1.2.3")
	flavor := fs.String("flavor", "standard", "standard or compact")
	notes := fs.String("notes", "", "what changed; cashiers see this after the update")
	keyPath := fs.String("key", defaultKeyPath(), "the release key")
	targetAll := fs.Bool("target-all", false, "same as -branches all")
	branches := fs.String("branches", "", `which branches' tills get it: "all", "none", or branch codes like CN,HR; asked when left out`)
	force := fs.Bool("force", false, "the tills install it right away, even during a shift")
	_ = fs.Parse(args)

	*version = strings.TrimPrefix(strings.TrimSpace(*version), "v")
	if !regexp.MustCompile(`^\d+\.\d+\.\d+(\.\d+)?$`).MatchString(*version) || *version == "0.0.0" {
		return fmt.Errorf("version %q isn't a release version; tag the commit first (git tag v1.2.3)", *version)
	}
	if *file == "" {
		return errors.New("-file is required")
	}
	baseURL := strings.TrimRight(os.Getenv("KASHI_URL"), "/")
	user := os.Getenv("KASHI_USER")
	if baseURL == "" || user == "" {
		return errors.New("set KASHI_URL (the tills' central URL, e.g. https://cloud.example.com/api) and KASHI_USER (a kashi admin)")
	}

	priv, err := loadPrivateKey(*keyPath)
	if err != nil {
		return err
	}
	// The key the tills check with is the one compiled into this repo; a
	// release signed with any other would be refused by every till.
	pub, err := base64.StdEncoding.DecodeString(updatekey.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("%s has no public key; run `make update-keygen` first", updateKeyFile)
	}
	if !bytes.Equal(pub, priv.Public().(ed25519.PublicKey)) {
		return fmt.Errorf("%s doesn't match the public key in %s; tills would refuse this release", *keyPath, updateKeyFile)
	}

	content, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	signature := ed25519.Sign(priv, digest[:])

	password := os.Getenv("KASHI_PASSWORD")
	if password == "" {
		fmt.Printf("kashi password for %s: ", user)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			// Not a terminal: read a line instead.
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			raw = []byte(strings.TrimSpace(line))
		}
		password = string(raw)
	}
	token, err := login(baseURL, user, password)
	if err != nil {
		return err
	}

	fmt.Printf("Uploading %s (%.1f MB) as GB Cloud %s (%s)…\n", filepath.Base(*file), float64(len(content))/(1<<20), *version, *flavor)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{
		"version":   *version,
		"flavor":    *flavor,
		"notes":     *notes,
		"sha256":    hex.EncodeToString(digest[:]),
		"signature": base64.StdEncoding.EncodeToString(signature),
	} {
		_ = w.WriteField(k, v)
	}
	part, err := w.CreateFormFile("file", filepath.Base(*file))
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/app_releases", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 15 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed (status %d): %s", resp.StatusCode, errorMessage(resp.Body))
	}
	var created struct {
		Release struct {
			ID int64 `json:"id"`
		} `json:"release"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || created.Release.ID == 0 {
		return errors.New("kashi's upload answer had no release id")
	}
	fmt.Printf("Published GB Cloud %s (%s).\n", *version, *flavor)
	if *targetAll {
		*branches = "all"
	}
	return targetBranches(baseURL, token, created.Release.ID, *version, *flavor, *branches, *force)
}

type register struct {
	ID         int64  `json:"id"`
	BranchCode string `json:"branch_code"`
	BranchName string `json:"branch_name"`
	Code       string `json:"code"`
	IsActive   bool   `json:"is_active"`
	AppFlavor  string `json:"app_flavor"`
	AppVersion string `json:"app_version"`
	Connected  bool   `json:"connected"`
}

type branch struct {
	Code, Name string
	Registers  []register
}

// targetBranches does what ticking tills on the App Updates page does: the
// active registers of the chosen branches that run this flavor are set to
// the release, and kashi tells the connected ones to fetch it now (the
// others pick it up when they next connect). choice is "all", "none", branch
// codes ("CN,HR"), or "" to ask. A register that hasn't reported its flavor
// yet is left alone and listed, since it may be the other build.
func targetBranches(baseURL, token string, releaseID int64, version, flavor, choice string, force bool) error {
	var list struct {
		Registers []register `json:"registers"`
	}
	if err := apiJSON(http.MethodGet, baseURL+"/app_updates/registers", token, nil, &list); err != nil {
		return fmt.Errorf("list registers: %w", err)
	}

	var branches []*branch
	byCode := map[string]*branch{}
	for _, r := range list.Registers {
		if !r.IsActive {
			continue
		}
		if r.AppFlavor == "" {
			fmt.Printf("Skipped %s/%s (hasn't reported which build it runs)\n", r.BranchCode, r.Code)
			continue
		}
		if r.AppFlavor != flavor {
			continue
		}
		b := byCode[r.BranchCode]
		if b == nil {
			b = &branch{Code: r.BranchCode, Name: r.BranchName}
			byCode[r.BranchCode] = b
			branches = append(branches, b)
		}
		b.Registers = append(b.Registers, r)
	}
	if len(branches) == 0 {
		fmt.Printf("No active %s till to send it to; set them on kashi's App Updates page.\n", flavor)
		return nil
	}

	if strings.TrimSpace(choice) == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Println("Choose which tills get it on kashi's App Updates page (or pass BRANCHES=…).")
			return nil
		}
		fmt.Printf("\nSend %s to which branches?\n", version)
		for i, b := range branches {
			fmt.Printf("  %d) %-6s %s\n", i+1, b.Code, b.Name)
			for _, r := range b.Registers {
				state := "offline, updates when it connects"
				if r.Connected {
					state = "online"
				}
				fmt.Printf("       %s runs %s, %s\n", r.Code, r.AppVersion, state)
			}
		}
		fmt.Print(`Numbers or codes separated by commas, "all", or Enter for none: `)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		choice = strings.TrimSpace(line)
		if choice == "" {
			choice = "none"
		}
	}
	chosen, err := pickBranches(branches, choice)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		fmt.Println("Not sent anywhere yet; choose tills on kashi's App Updates page.")
		return nil
	}

	var ids []int64
	var lines []string
	for _, b := range chosen {
		for _, r := range b.Registers {
			ids = append(ids, r.ID)
			lines = append(lines, fmt.Sprintf("  %s/%s (runs %s)", b.Code, r.Code, r.AppVersion))
		}
	}
	body := map[string]any{"release_id": releaseID, "register_ids": ids, "force": force}
	if err := apiJSON(http.MethodPut, baseURL+"/app_updates/target", token, body, nil); err != nil {
		return fmt.Errorf("set target: %w", err)
	}
	fmt.Printf("Sent %s to %d till(s):\n%s\n", version, len(ids), strings.Join(lines, "\n"))
	if force {
		fmt.Println("Forced: each till installs it as soon as it has it, after a one-minute countdown.")
	}
	fmt.Println("kashi deletes the build from the server once they've all installed it. Follow them on the App Updates page.")
	return nil
}

// pickBranches reads "all", "none", or a comma-separated list of branch
// numbers (as listed) and/or codes.
func pickBranches(branches []*branch, choice string) ([]*branch, error) {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "all":
		return branches, nil
	case "none", "":
		return nil, nil
	}
	var out []*branch
	seen := map[*branch]bool{}
	for _, part := range strings.Split(choice, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var match *branch
		if n, err := strconv.Atoi(part); err == nil && n >= 1 && n <= len(branches) {
			match = branches[n-1]
		}
		for _, b := range branches {
			if match == nil && strings.EqualFold(b.Code, part) {
				match = b
			}
		}
		if match == nil {
			return nil, fmt.Errorf("no branch %q with a till of this build; nothing was sent (the release is published, choose tills on the App Updates page)", part)
		}
		if !seen[match] {
			seen[match] = true
			out = append(out, match)
		}
	}
	return out, nil
}

// apiJSON sends body (if any) as JSON and decodes the answer into out (if
// any).
func apiJSON(method, url, token string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, errorMessage(resp.Body))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func login(baseURL, user, password string) (string, error) {
	raw, _ := json.Marshal(map[string]string{"name": user, "password": password})
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Post(baseURL+"/users/login", "application/json", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("log in to kashi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("log in to kashi failed (status %d): %s", resp.StatusCode, errorMessage(resp.Body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.AccessToken == "" {
		return "", errors.New("kashi's login answer had no token")
	}
	return out.AccessToken, nil
}

func errorMessage(r io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(r, 4096))
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &body) == nil && body.Error != "" {
		return body.Error
	}
	return strings.TrimSpace(string(raw))
}
