// Command urthbot is a headless client for driving a running server from
// scripts and tests: it connects over TCP, logs in (creating the character
// if needed), sends each command, and prints the reply with color stripped.
//
//	urthbot -addr 127.0.0.1:4000 -name bob -password secret5 look north 'say hi'
//	urthbot -name bob -password secret5 -stdin < commands.txt
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

// clean strips ANSI color and telnet IAC sequences (raw bytes that Go's
// regexp will not accept as a pattern).
func clean(s string) string {
	b := []byte(s)
	out := b[:0]
	for i := 0; i < len(b); i++ {
		if b[i] == 0xff && i+1 < len(b) {
			if b[i+1] >= 0xfb && b[i+1] <= 0xfe {
				i += 2
				continue
			}
			i++
			continue
		}
		out = append(out, b[i])
	}
	return ansi.ReplaceAllString(string(out), "")
}

func main() {
	addr := flag.String("addr", "127.0.0.1:4000", "server address")
	name := flag.String("name", "", "character name (required)")
	password := flag.String("password", "", "password (required)")
	wait := flag.Duration("wait", 300*time.Millisecond, "how long to collect output after each command")
	useStdin := flag.Bool("stdin", false, "read commands from stdin, one per line")
	quiet := flag.Bool("quiet", false, "print only command output, not the command")
	stay := flag.Duration("stay", 0, "after the last command, keep collecting output for this long")
	flag.Parse()
	if *name == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "urthbot: -name and -password are required")
		os.Exit(2)
	}

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "urthbot:", err)
		os.Exit(1)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	read := func(d time.Duration) string {
		_ = conn.SetReadDeadline(time.Now().Add(d))
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return clean(b.String())
	}
	send := func(line string) string {
		fmt.Fprint(conn, line+"\r\n")
		return read(*wait)
	}

	// Login: name, then either the password prompt or new-character flow.
	read(*wait)
	out := send(*name)
	switch {
	case strings.Contains(out, "Password:"):
		out = send(*password)
		if !strings.Contains(out, "Welcome") && !strings.Contains(out, "Reconnecting") {
			// Hashing is async; give it a moment.
			out += read(2 * time.Second)
		}
	case strings.Contains(out, "(Y/N)"):
		send("y")
		send(*password)
		out = send(*password)
		if !strings.Contains(out, "Welcome") {
			out += read(2 * time.Second)
		}
	}
	if !strings.Contains(out, "Welcome") && !strings.Contains(out, "Reconnecting") {
		fmt.Fprintf(os.Stderr, "urthbot: login failed:\n%s\n", out)
		os.Exit(1)
	}

	var cmds []string
	if *useStdin {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			cmds = append(cmds, sc.Text())
		}
	}
	cmds = append(cmds, flag.Args()...)
	for _, c := range cmds {
		if !*quiet {
			fmt.Printf("> %s\n", c)
		}
		fmt.Print(strings.ReplaceAll(send(c), "\r\n", "\n"))
	}
	if *stay > 0 {
		fmt.Print(strings.ReplaceAll(read(*stay), "\r\n", "\n"))
	}
}
