package system

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// TestReadRegularFileRefusesWhatIsNotARegularFile pins WB-B4 (bugs.md
// Q-WB-5): a FIFO, even behind a symlink, is refused at once instead of
// blocking the read until something writes to it, and so are a directory
// and a device, whose read could fail late or never end.
func TestReadRegularFileRefusesWhatIsNotARegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(fifo, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fifo, link, dir, os.DevNull} {
		done := make(chan error, 1)
		go func() {
			_, err := FS{}.ReadRegularFile(path, 1<<20)
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, notRegularError{}) || err.Error() != "not a regular file" {
				t.Errorf("ReadRegularFile(%s) = %v; want notRegularError", path, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("ReadRegularFile(%s) blocked", path)
		}
	}
}

func TestReadRegularFileReadsAFileAndReportsAMissingOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("image: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, link} {
		if data, err := (FS{}).ReadRegularFile(path, 1<<20); err != nil || string(data) != "image: {}\n" {
			t.Errorf("ReadRegularFile(%s) = %q, %v; want the contents", path, data, err)
		}
	}
	if _, err := (FS{}).ReadRegularFile(filepath.Join(dir, "missing"), 1<<20); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadRegularFile(missing) = %v; want fs.ErrNotExist", err)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.WriteFile(locked, nil, 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := (FS{}).ReadRegularFile(locked, 1<<20); os.Geteuid() != 0 && !errors.Is(err, fs.ErrPermission) {
		t.Errorf("ReadRegularFile(locked) = %v; want fs.ErrPermission", err)
	}
}

// TestReadRegularFileNeverOpensAFIFO: a FIFO is refused before it is opened,
// so a writer waiting for a reader (a container's process, say) is not let
// through by caged's check.
func TestReadRegularFileNeverOpensAFIFO(t *testing.T) {
	t.Parallel()

	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	opened := make(chan *os.File, 1)
	go func() {
		writer, err := os.OpenFile(fifo, os.O_WRONLY, 0) // blocks until a reader opens the FIFO
		if err == nil {
			opened <- writer
		}
	}()
	time.Sleep(200 * time.Millisecond) // let the writer block in open
	if _, err := (FS{}).ReadRegularFile(fifo, 1<<20); !errors.Is(err, notRegularError{}) {
		t.Fatalf("ReadRegularFile() = %v; want notRegularError", err)
	}
	select {
	case writer := <-opened:
		_ = writer.Close()
		t.Fatal("ReadRegularFile opened the FIFO: the waiting writer got through")
	case <-time.After(300 * time.Millisecond):
	}
	reader, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0) // let the writer go
	if err != nil {
		t.Fatal(err)
	}
	_ = (<-opened).Close()
	_ = reader.Close()
}

// TestReadRegularFileHoldsWhenAFIFOIsSwappedIn: a FIFO renamed over the file
// between the check and the open is still refused without blocking, because
// the open does not block and the opened file is checked again. Every read
// returns the regular file's contents or notRegularError, never a hang or the
// FIFO's empty read.
func TestReadRegularFileHoldsWhenAFIFOIsSwappedIn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	swapped := make(chan error, 1)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				swapped <- nil
				return
			default:
			}
			next := filepath.Join(dir, "next")
			var err error
			if i%2 == 0 {
				err = syscall.Mkfifo(next, 0o600)
			} else {
				err = os.WriteFile(next, []byte("x\n"), 0o600)
			}
			if err == nil {
				err = os.Rename(next, path)
			}
			if err != nil {
				swapped <- err
				return
			}
		}
	}()
	// done carries the first wrong result, or "" when every read was right.
	done := make(chan string, 1)
	go func() {
		for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
			data, err := (FS{}).ReadRegularFile(path, 1<<20)
			if err != nil && !errors.Is(err, notRegularError{}) || err == nil && string(data) != "x\n" {
				done <- fmt.Sprintf("ReadRegularFile() = %q, %v; want the file or notRegularError", data, err)
				return
			}
		}
		done <- ""
	}()
	select {
	case failure := <-done:
		if failure != "" {
			t.Error(failure)
		}
	case <-time.After(10 * time.Second):
		t.Error("ReadRegularFile blocked on a FIFO swapped in after the check")
	}
	close(stop)
	if err := <-swapped; err != nil {
		t.Fatal(err)
	}
	if reader, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil { // release a read the test gave up on
		_ = reader.Close()
	}
}

// TestReadRegularFileRefusesAFileOverTheLimit pins WB-B6 (bugs.md Q-WB-9): a
// container can make config.yaml a sparse file of any size; it is refused
// after reading one byte more than the limit, and a file at the limit is
// still read.
func TestReadRegularFileRefusesAFileOverTheLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for size, wantErr := range map[int64]bool{1 << 40: true, 101: true, 100: false} {
		path := filepath.Join(dir, fmt.Sprint(size))
		f, err := os.Create(path)
		if err == nil {
			err = errors.Join(f.Truncate(size), f.Close())
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := (FS{}).ReadRegularFile(path, 100)
		var tooLarge tooLargeError
		if wantErr && (!errors.As(err, &tooLarge) || tooLarge.limit != 100 || err.Error() != "larger than 100 bytes") || !wantErr && (err != nil || len(data) != 100) {
			t.Errorf("ReadRegularFile(%d bytes, 100) = %d bytes, %v", size, len(data), err)
		}
	}
}

// TestReadRegularFileStopsAFileThatGrowsWhileItIsRead: a file the container
// grows while it is read never returns more than the limit.
func TestReadRegularFileStopsAFileThatGrowsWhileItIsRead(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stop, grown := make(chan struct{}), make(chan error, 1)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				grown <- nil
				return
			default:
			}
			if err := os.Truncate(path, int64(2+(i%2)*(1<<16))); err != nil {
				grown <- err
				return
			}
		}
	}()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		data, err := (FS{}).ReadRegularFile(path, 1<<10)
		if err != nil && !errors.As(err, new(tooLargeError)) || len(data) > 1<<10 {
			t.Errorf("ReadRegularFile() = %d bytes, %v; want at most the limit or tooLargeError", len(data), err)
			break
		}
	}
	close(stop)
	if err := <-grown; err != nil {
		t.Fatal(err)
	}
}

// TestReadRegularFileReadsNoMoreThanTheLimit: refusing a huge file costs
// about the limit in memory, not the file's size. Not parallel, so the
// allocation count is this test's own.
func TestReadRegularFileReadsNoMoreThanTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sparse")
	f, err := os.Create(path)
	if err == nil {
		err = errors.Join(f.Truncate(64<<20), f.Close())
	}
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = (FS{}).ReadRegularFile(path, 100)
	runtime.ReadMemStats(&after)
	if !errors.As(err, new(tooLargeError)) || after.TotalAlloc-before.TotalAlloc > 1<<20 {
		t.Fatalf("ReadRegularFile(64 MiB, 100) = %v after allocating %d bytes; want tooLargeError and well under 1 MiB", err, after.TotalAlloc-before.TotalAlloc)
	}
}
