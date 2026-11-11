package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// The publisher's messages: one short coloured line per step, and errors that
// say what happened and what to do next. Colour only on a terminal (never with
// NO_COLOR set; FORCE_COLOR=1 forces it), like scripts/pretty.sh.
const (
	cReset = "\033[0m"
	cDim   = "\033[2m"
	cBold  = "\033[1m"
	cRed   = "\033[31m"
)

func colorOn(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// paint wraps text in a colour when the terminal takes it.
func paint(on bool, color, text string) string {
	if !on {
		return text
	}
	return color + text + cReset
}

// badge is a message's lead: dark text on a coloured background on a
// terminal (the same look as scripts/pretty.sh), the plain mark otherwise.
func badge(on bool, mark, word, style string) string {
	if !on {
		return mark
	}
	return "\033[" + style + "m " + word + " " + cReset
}

func line(f *os.File, mark, word, style, text string) {
	fmt.Fprintf(f, "%s %s\n", badge(colorOn(f), mark, word, style), text)
}

func step(format string, a ...any) {
	line(os.Stdout, "▸", "RUN", "1;30;46", fmt.Sprintf(format, a...))
}
func ok(format string, a ...any) { line(os.Stdout, "✔", "OK", "1;30;42", fmt.Sprintf(format, a...)) }
func warn(format string, a ...any) {
	line(os.Stdout, "!", "WARN", "1;30;43", fmt.Sprintf(format, a...))
}
func fail(text string) {
	on := colorOn(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s %s\n", badge(on, "✘", "FAIL", "1;97;41"), paint(on, cBold+cRed, text))
}
func dim(format string, a ...any) {
	fmt.Println(paint(colorOn(os.Stdout), cDim, fmt.Sprintf(format, a...)))
}

// uploadFailure explains a refused upload: what it means and what to do, in
// place of "upload failed (status 502): error code: 502".
func uploadFailure(status int, answer, version, flavor string) string {
	detail := strings.TrimSpace(answer)
	switch {
	case status == http.StatusConflict:
		return fmt.Sprintf("GB Cloud %s (%s) is already published, so nothing was uploaded. Bump the version (make bump) and publish again; if it was your last run that uploaded it, you're done.", version, flavor)
	case status == http.StatusBadGateway, status == http.StatusServiceUnavailable, status == http.StatusGatewayTimeout, status >= 520 && status <= 523:
		return fmt.Sprintf("Central isn't answering right now (status %d): it is most likely being updated. Nothing was published; wait a minute, then run the same command again.", status)
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return fmt.Sprintf("Central refused the upload (status %d): the account may not be allowed to publish. %s", status, detail)
	case status == http.StatusRequestEntityTooLarge:
		return "Central refused the upload as too big (status 413). Its web server's upload limit needs raising."
	case status == http.StatusBadRequest:
		return fmt.Sprintf("Central didn't accept the upload (status 400): %s", detail)
	}
	return fmt.Sprintf("Central refused the upload (status %d): %s", status, detail)
}
