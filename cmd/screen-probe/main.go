// Command screen-probe pokes a serial customer-display (the GSAN 7-segment pole
// display) with a battery of test sequences so you can watch the glass and learn
// what the firmware actually supports: decimal handling, digit alignment,
// brightness, pseudo-letters, and any word/annunciator control.
//
// Usage:
//
//	go run ./cmd/screen-probe -port COM3            # Windows
//	go run ./cmd/screen-probe -port /dev/ttyUSB0    # Linux
//	go run ./cmd/screen-probe -port COM3 -baud 9600 # try a different baud
//
// It runs a guided sequence (press Enter between steps, note what you see), then
// drops into an interactive prompt for free-form probing.
package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"go.bug.st/serial"
)

var (
	portName = flag.String("port", "", "serial port (e.g. COM3 or /dev/ttyUSB0)")
	baud     = flag.Int("baud", 2400, "baud rate (matches serial.go default; try 9600 if nothing shows)")
	gapMS    = flag.Int("gap", 60, "milliseconds to wait after an init/clear before writing")
)

var port serial.Port

func main() {
	flag.Parse()
	if *portName == "" {
		fmt.Println("need -port; available ports:")
		if ps, err := serial.GetPortsList(); err == nil {
			for _, p := range ps {
				fmt.Println("  ", p)
			}
		}
		os.Exit(2)
	}

	p, err := serial.Open(*portName, &serial.Mode{
		BaudRate: *baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		fmt.Println("open:", err)
		os.Exit(1)
	}
	defer p.Close()
	port = p
	fmt.Printf("opened %s @ %d 8N1\n\n", *portName, *baud)

	in := bufio.NewScanner(os.Stdin)
	step := func(title string, do func()) {
		fmt.Printf("── %s\n   (Enter to send, watch the display) ", title)
		in.Scan()
		do()
		fmt.Println()
	}

	// Control bytes.
	const (
		ESC = 0x1B
		CLR = 0x0C // form feed: clear on most VFD/LED displays
		CAN = 0x18 // cancel: clear current line
	)
	initDisp := func() {
		send([]byte{ESC, '@'})
		time.Sleep(time.Duration(*gapMS) * time.Millisecond)
	}

	fmt.Println("=== GUIDED PROBES ===")
	fmt.Println("For each step, note: did it change? where did digits land? any flicker?")
	fmt.Println()

	step("1. init (ESC @) then stream 8 digits: 12345678", func() {
		initDisp()
		send([]byte("12345678"))
	})

	step("2. more than 8 digits: 1234567890 (does it drop leading or trailing?)", func() {
		initDisp()
		send([]byte("1234567890"))
	})

	step("3. all segments: 88888888 (spot any dead segment)", func() {
		initDisp()
		send([]byte("88888888"))
	})

	step("4. decimal: 12.34 (does '.' use the dot segment or eat a digit cell?)", func() {
		initDisp()
		send([]byte("12.34"))
	})

	step("5. decimal wide: 100.00", func() {
		initDisp()
		send([]byte("100.00"))
	})

	step("6. leading spaces: '    56.78' (does it right-justify / accept spaces?)", func() {
		initDisp()
		send([]byte("    56.78"))
	})

	step("7. minus sign: -12.34", func() {
		initDisp()
		send([]byte("-12.34"))
	})

	step("8. form-feed clear (0x0C) then 4321", func() {
		send([]byte{CLR})
		time.Sleep(time.Duration(*gapMS) * time.Millisecond)
		send([]byte("4321"))
	})

	step("9. CAN clear (0x18) then 777", func() {
		send([]byte{CAN})
		time.Sleep(time.Duration(*gapMS) * time.Millisecond)
		send([]byte("777"))
	})

	step("10. pseudo-letters on digit row: ABCDEF (expected: garbage, per your test)", func() {
		initDisp()
		send([]byte("ABCDEF"))
	})

	step("11. pseudo-letters lowercase-ish: PtoCHLnU", func() {
		initDisp()
		send([]byte("PtoCHLnU"))
	})

	step("12. brightness ESC * 1 (dim) then 88888888", func() {
		send([]byte{ESC, '*', 1})
		time.Sleep(time.Duration(*gapMS) * time.Millisecond)
		send([]byte("88888888"))
	})
	step("13. brightness ESC * 4 (bright) then 88888888", func() {
		send([]byte{ESC, '*', 4})
		time.Sleep(time.Duration(*gapMS) * time.Millisecond)
		send([]byte("88888888"))
	})

	// CD5220 string mode: ESC Q A <chars> CR  (upper line, overwrite).
	step("14. CD5220 string mode: ESC Q A '12345678' CR", func() {
		initDisp()
		send([]byte{ESC, 'Q', 'A'})
		send([]byte("12345678"))
		send([]byte{0x0D})
	})

	// DM-D style overwrite + cursor home, then positioned writes. On a numeric
	// panel these often no-op, but if positioning works you can push a value
	// under a specific printed word.
	step("15. DM-D: ESC '=' (select display) , 'US B' home, write 42", func() {
		initDisp()
		send([]byte{ESC, '='})
		send([]byte{0x1F, 'B'}) // US B: some panels = cursor home
		send([]byte("42"))
	})
	step("16. try cursor to a right position then '9': ESC 'l' 8  (guess)", func() {
		initDisp()
		send([]byte{ESC, 'l', 8})
		send([]byte("9"))
	})

	// Speculative word/annunciator pokes. There is NO standard for this on the
	// ESC/POS numeric set; these are documented guesses. If none change the
	// printed words, the words are silkscreen with no light element.
	fmt.Println("── ANNUNCIATOR GUESSES (no standard exists; likely no effect) ──")
	for _, g := range [][]byte{
		{ESC, 's', 0x01},
		{ESC, 's', 0x0F},
		{ESC, 'S', 0x01},
		{ESC, 'Y', 0x01},
		{ESC, 0x23, 0x01}, // ESC #
		{0x1F, 'A', 0x01},
	} {
		step(fmt.Sprintf("    send % X (then 88888888)", g), func() {
			initDisp()
			send(g)
			time.Sleep(time.Duration(*gapMS) * time.Millisecond)
			send([]byte("88888888"))
		})
	}

	step("17. final clear", func() { send([]byte{ESC, '@'}) })

	fmt.Println()
	fmt.Println("=== INTERACTIVE MODE ===")
	fmt.Println("Commands:")
	fmt.Println("  <hex bytes>   e.g.  1B 40 31 32 33     -> send raw bytes")
	fmt.Println("  s <text>      e.g.  s 12.34            -> send ASCII text")
	fmt.Println("  i             -> send ESC @ (init/clear)")
	fmt.Println("  c             -> send 0x0C (form-feed clear)")
	fmt.Println("  8             -> send 88888888")
	fmt.Println("  q             -> quit")
	fmt.Println()

	rd := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("probe> ")
		line, err := rd.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case line == "q":
			return
		case line == "i":
			send([]byte{ESC, '@'})
		case line == "c":
			send([]byte{CLR})
		case line == "8":
			send([]byte("88888888"))
		case strings.HasPrefix(line, "s "):
			send([]byte(line[2:]))
		default:
			raw := strings.ReplaceAll(line, " ", "")
			b, err := hex.DecodeString(raw)
			if err != nil {
				fmt.Println("  not valid hex and not a known command:", err)
				continue
			}
			send(b)
		}
	}
}

func send(b []byte) {
	n, err := port.Write(b)
	if err != nil {
		fmt.Printf("  write error after %d bytes: %v\n", n, err)
		return
	}
	fmt.Printf("  sent % X  (%q)\n", b, printable(b))
	time.Sleep(30 * time.Millisecond)
}

func printable(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c >= 0x20 && c < 0x7f {
			sb.WriteByte(c)
		} else {
			sb.WriteByte('.')
		}
	}
	return sb.String()
}
