package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"urth/internal/config"
	"urth/internal/store"
)

// runAdmin is `urth admin`: account maintenance against the player files
// on the host, for when nobody with admin is logged in, or nobody can log
// in at all. It edits files directly, so for a character who is online
// right now the in-game command is the one to use: the server writes its
// own copy of the record at the next save.
func runAdmin(args []string) error {
	fs := flag.NewFlagSet("urth admin", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path to the server configuration file")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `usage: urth admin [-config path] <command> [name]

  list             every character, with admin and denied marks
  show <name>      the record, minus the password hash
  promote <name>   make a character an admin
  demote <name>    take admin away
  passwd <name>    set a new password, read from standard input
  deny <name>      lock the account out
  allow <name>     undo deny
  delete <name>    retire the character; the file moves to players/deleted/

If the character is online, use the in-game command instead: the server
overwrites the file at its next save.`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("a command is required")
	}
	cfg, _, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	st, err := store.New(filepath.Join(cfg.Paths.Data, "players"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	cmd := fs.Arg(0)
	if cmd == "list" {
		names, err := st.List()
		if err != nil {
			return err
		}
		for _, n := range names {
			rec, err := st.Load(n)
			if err != nil {
				fmt.Printf("%-14s unreadable: %v\n", n, err)
				continue
			}
			marks := ""
			if rec.Admin {
				marks += " admin"
			}
			if rec.Denied {
				marks += " denied"
			}
			fmt.Printf("%-14s level %-3d last login %s%s\n", n, rec.Level, rec.LastLogin.Local().Format("2006-01-02 15:04"), marks)
		}
		fmt.Printf("%d characters.\n", len(names))
		return nil
	}

	if fs.NArg() < 2 {
		return fmt.Errorf("%s needs a character name", cmd)
	}
	name := store.Canonical(fs.Arg(1))
	if !store.ValidName(name) {
		return fmt.Errorf("%q is not a valid name", fs.Arg(1))
	}
	rec, err := st.Load(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	edit := func(what string, fn func()) error {
		fn()
		if err := st.Save(rec); err != nil {
			return err
		}
		fmt.Printf("%s: %s.\n", name, what)
		return nil
	}
	switch cmd {
	case "show":
		fmt.Printf("name: %s\nadmin: %v\ndenied: %v\nlevel: %d\nroom: %d\ncreated: %s\nlast login: %s\n",
			rec.Name, rec.Admin, rec.Denied, rec.Level, rec.Room, rec.Created.Local().Format("2006-01-02 15:04"), rec.LastLogin.Local().Format("2006-01-02 15:04"))
		return nil
	case "promote":
		return edit("now an admin", func() { rec.Admin = true })
	case "demote":
		return edit("no longer an admin", func() { rec.Admin = false })
	case "deny":
		return edit("denied", func() { rec.Denied = true })
	case "allow":
		return edit("allowed", func() { rec.Denied = false })
	case "passwd":
		pw, err := readPassword("New password for " + name + ": ")
		if err != nil {
			return err
		}
		if len(pw) < 5 || len(pw) > 64 {
			return errors.New("password must be 5 to 64 characters")
		}
		hash, err := st.HashPassword(pw)
		if err != nil {
			return err
		}
		return edit("password changed", func() { rec.PasswordHash = hash })
	case "delete":
		if err := st.Delete(name); err != nil {
			return err
		}
		fmt.Printf("%s: moved to players/deleted/.\n", name)
		return nil
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

// readPassword reads one line from standard input. On a terminal the
// echo is turned off with stty so the password does not show; from a pipe
// it just reads the line.
func readPassword(prompt string) (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	terminal := info.Mode()&os.ModeCharDevice != 0
	if terminal {
		fmt.Print(prompt)
		if err := stty("-echo"); err == nil {
			defer func() {
				_ = stty("echo")
				fmt.Println()
			}()
		}
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func stty(arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
