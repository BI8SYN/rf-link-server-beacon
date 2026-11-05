package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicStateRecoveryAndLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, e := OpenStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	if other, e := OpenStore(dir); e == nil {
		other.Close()
		t.Fatal("second instance allowed")
	}
	defer s.Close()
	type record struct {
		RequestID string `json:"requestId"`
	}
	if e = s.Write(record{"durable"}); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, ".unfinished"), []byte("broken"), 0600)
	var got record
	if e = s.Read(&got); e != nil || got.RequestID != "durable" {
		t.Fatal(got, e)
	}
	info, _ := os.Stat(filepath.Join(dir, "state.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("state permissions")
	}
}

func TestRejectSharedDirectoryAndCorruptState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	os.Mkdir(dir, 0755)
	if s, e := OpenStore(dir); e == nil {
		s.Close()
		t.Fatal("shared state directory accepted")
	}
	os.Chmod(dir, 0700)
	s, e := OpenStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"schema":1,"schema":2}`), 0600)
	var v struct {
		Schema uint64 `json:"schema"`
	}
	if e = s.Read(&v); e == nil {
		t.Fatal("corrupt state accepted")
	}
}
