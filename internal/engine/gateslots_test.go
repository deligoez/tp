package engine

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The helper process holds or waits for a slot in a real second process, so
// the kill tests measure what the kernel does with a dead holder's lock rather
// than what an in-process unlock does.
const (
	envSlotHelperDir  = "TP_TEST_GATE_SLOT_DIR"
	envSlotHelperCap  = "TP_TEST_GATE_SLOT_CAP"
	envSlotHelperMain = "TP_TEST_GATE_SLOT_MAIN"
)

// TestGateSlotHelperProcess is not a test: run by startSlotHelper, it takes a
// slot, says "held", and blocks until it is killed.
func TestGateSlotHelperProcess(t *testing.T) {
	dir := os.Getenv(envSlotHelperDir)
	if dir == "" {
		t.Skip("helper process only")
	}
	capN, _ := strconv.Atoi(os.Getenv(envSlotHelperCap))
	slot, err := AcquireGateSlot(context.Background(), dir, capN, os.Getenv(envSlotHelperMain) == "1")
	if err != nil {
		os.Exit(3)
	}
	defer slot.Release()
	_, _ = os.Stdout.WriteString("held\n")
	time.Sleep(time.Hour)
}

// startSlotHelper starts the helper; held is closed once it holds its slot.
func startSlotHelper(t *testing.T, dir string, capN int, main bool) (cmd *exec.Cmd, held <-chan struct{}) {
	t.Helper()
	mainFlag := "0"
	if main {
		mainFlag = "1"
	}
	cmd = exec.Command(os.Args[0], "-test.run=^TestGateSlotHelperProcess$") //nolint:gosec // the test binary itself
	cmd.Env = append(os.Environ(), envSlotHelperDir+"="+dir, envSlotHelperCap+"="+strconv.Itoa(capN), envSlotHelperMain+"="+mainFlag)
	out, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	ch := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if sc.Text() == "held" {
				close(ch)
				return
			}
		}
	}()
	return cmd, ch
}

// killHelper SIGKILLs the helper and reaps it, so its locks are gone when it
// returns.
func killHelper(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	require.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()
}

// acquireWithin takes a slot or gives up after d.
func acquireWithin(dir string, capN int, main bool, d time.Duration) (*GateSlot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return AcquireGateSlot(ctx, dir, capN, main)
}

// TestGateSlots_TheCapIsNeverExceededMainIncluded: six gates, one of them the
// main branch's, never hold more than the two slots at once, and do hold both.
func TestGateSlots_TheCapIsNeverExceededMainIncluded(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "gate-slots")
	var active, peak atomic.Int32
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Go(func() {
			slot, err := acquireWithin(dir, 2, i == 0, 10*time.Second)
			if !assert.NoError(t, err) {
				return
			}
			n := active.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(80 * time.Millisecond)
			active.Add(-1)
			slot.Release()
		})
	}
	wg.Wait()
	assert.Equal(t, int32(2), peak.Load(), "the gates held both slots at once and never a third")
}

// TestGateSlots_AKilledHolderFreesItsSlotAtOnce: a process killed while it
// holds the only slot frees it with its death, with no cleanup by anyone.
func TestGateSlots_AKilledHolderFreesItsSlotAtOnce(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "gate-slots")
	cmd, held := startSlotHelper(t, dir, 1, false)
	<-held

	_, err := acquireWithin(dir, 1, false, 300*time.Millisecond)
	require.Error(t, err, "a held slot lets no second gate in")

	killHelper(t, cmd)
	start := time.Now()
	slot, err := acquireWithin(dir, 1, false, 5*time.Second)
	require.NoError(t, err, "the dead holder's slot is free")
	slot.Release()
	assert.Less(t, time.Since(start), time.Second, "freed at once, not after a timeout")
}
