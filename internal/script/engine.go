// Package script embeds goja and exposes the rule hooks. Every game formula
// lives in data/scripts/*.js; the engine loads them into one runtime, calls
// named global functions, and reloads when a file changes. Scripts return
// plain values; they never mutate game state. See docs/RULES.md section 1.
package script

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// DefaultBudget is how long one hook call may run before it is killed.
const DefaultBudget = 50 * time.Millisecond

// ErrNoHook is returned when the scripts do not define the function.
var ErrNoHook = errors.New("hook not defined")

// Engine owns the runtime. All calls happen on the world goroutine; the
// mutex only guards against Reload racing a call from an admin command
// handled elsewhere in future.
type Engine struct {
	dir    string
	log    *slog.Logger
	budget time.Duration

	mu     sync.Mutex
	vm     *goja.Runtime
	mtimes map[string]time.Time
	// failed holds the mtimes of the last set that did not compile, so a
	// broken file is reported once, not every check.
	failed map[string]time.Time
	files  []string
	rng    *rand.Rand
	// loadedAt is when the current runtime was built.
	loadedAt time.Time
}

// New creates an engine over dir. Call Load before using it.
func New(dir string, log *slog.Logger, budget time.Duration) *Engine {
	if budget <= 0 {
		budget = DefaultBudget
	}
	return &Engine{dir: dir, log: log, budget: budget, mtimes: map[string]time.Time{},
		rng: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
}

// SetRandom replaces the random source scripts see. Simulations pass a
// seeded one so runs are reproducible.
func (e *Engine) SetRandom(r *rand.Rand) {
	e.mu.Lock()
	e.rng = r
	e.mu.Unlock()
}

// Files lists the scripts currently loaded.
func (e *Engine) Files() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.files...)
}

// LoadedAt is when the current scripts were compiled.
func (e *Engine) LoadedAt() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.loadedAt
}

// Load compiles every *.js in dir, in name order, into a fresh runtime and
// swaps it in. On any error the previous runtime is kept.
func (e *Engine) Load() error {
	files, err := filepath.Glob(filepath.Join(e.dir, "*.js"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no scripts in %s", e.dir)
	}
	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	e.install(vm)
	mtimes := map[string]time.Time{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		info, err := os.Stat(f)
		if err != nil {
			return err
		}
		if _, err := vm.RunScript(f, string(src)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		mtimes[f] = info.ModTime()
	}
	e.mu.Lock()
	e.vm = vm
	e.mtimes = mtimes
	e.files = files
	e.loadedAt = time.Now()
	e.mu.Unlock()
	e.log.Info("scripts loaded", "files", len(files))
	return nil
}

// Reload checks the scripts' modification times and reloads if any changed
// or were added or removed since the last successful load or failed
// attempt. It reports whether a reload was attempted.
func (e *Engine) Reload() (bool, error) {
	files, err := filepath.Glob(filepath.Join(e.dir, "*.js"))
	if err != nil {
		return false, err
	}
	current := map[string]time.Time{}
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		current[f] = info.ModTime()
	}
	e.mu.Lock()
	changed := !sameTimes(current, e.mtimes) && !sameTimes(current, e.failed)
	e.mu.Unlock()
	if !changed {
		return false, nil
	}
	if err := e.Load(); err != nil {
		e.mu.Lock()
		e.failed = current
		e.mu.Unlock()
		return true, err
	}
	e.mu.Lock()
	e.failed = nil
	e.mu.Unlock()
	return true, nil
}

func sameTimes(a, b map[string]time.Time) bool {
	if b == nil || len(a) != len(b) {
		return false
	}
	for f, t := range a {
		if !t.Equal(b[f]) {
			return false
		}
	}
	return true
}

// install binds the small API scripts may call.
func (e *Engine) install(vm *goja.Runtime) {
	_ = vm.Set("random", map[string]any{
		// int(n) returns 0..n-1.
		"int": func(n int) int {
			if n <= 0 {
				return 0
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			return e.rng.IntN(n)
		},
		// float() returns [0,1).
		"float": func() float64 {
			e.mu.Lock()
			defer e.mu.Unlock()
			return e.rng.Float64()
		},
		// roll(count, sides) sums dice.
		"roll": func(count, sides int) int {
			e.mu.Lock()
			defer e.mu.Unlock()
			total := 0
			for i := 0; i < count; i++ {
				total += 1 + e.rng.IntN(max(sides, 1))
			}
			return total
		},
	})
	_ = vm.Set("log", func(args ...any) {
		e.log.Info("script", "msg", fmt.Sprint(args...))
	})
}

// Has reports whether the scripts define a callable named name.
func (e *Engine) Has(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.vm == nil {
		return false
	}
	_, ok := goja.AssertFunction(e.vm.Get(name))
	return ok
}

// Call invokes the named hook with args and exports the result into out
// (a pointer to a struct with json tags, or to a basic type). A call that
// exceeds the budget is interrupted and reported as an error.
func (e *Engine) Call(name string, out any, args ...any) error {
	e.mu.Lock()
	vm := e.vm
	e.mu.Unlock()
	if vm == nil {
		return errors.New("scripts not loaded")
	}
	fn, ok := goja.AssertFunction(vm.Get(name))
	if !ok {
		return fmt.Errorf("%s: %w", name, ErrNoHook)
	}
	values := make([]goja.Value, len(args))
	for i, a := range args {
		values[i] = vm.ToValue(a)
	}
	timer := time.AfterFunc(e.budget, func() { vm.Interrupt("time budget exceeded") })
	res, err := fn(goja.Undefined(), values...)
	timer.Stop()
	vm.ClearInterrupt()
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if out == nil {
		return nil
	}
	if err := vm.ExportTo(res, out); err != nil {
		return fmt.Errorf("%s: bad return value: %w", name, err)
	}
	return nil
}
