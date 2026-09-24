package copyover

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestStateRoundTripIsSingleUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "copyover.json")
	st := State{
		Listeners: []Listener{{Kind: "telnet", FD: 3}},
		Players:   []Player{{Kind: "telnet", FD: 7, Name: "Bob", Room: 2}, {Kind: "web", Token: "abc", Name: "Alice", Room: 1}},
	}
	if err := Write(path, st); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode %o", info.Mode().Perm())
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Players) != 2 || got.Players[0].FD != 7 || got.Players[1].Token != "abc" || got.Listeners[0].Kind != "telnet" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("state file should be removed after Read")
	}
}

func TestInheritClearsCloseOnExec(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "fd")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fd := int(f.Fd())
	flags, _, _ := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
	if flags&syscall.FD_CLOEXEC == 0 {
		t.Fatal("expected Go to open files close-on-exec")
	}
	if err := Inherit(fd); err != nil {
		t.Fatal(err)
	}
	flags, _, _ = syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
	if flags&syscall.FD_CLOEXEC != 0 {
		t.Fatal("close-on-exec still set")
	}
}
