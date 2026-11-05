package app

import (
	"encoding/json"
	"errors"
	"github.com/BI8SYN/rf-link-server-beacon/internal/jsonx"
	"os"
	"path/filepath"
	"syscall"
)

type Store struct {
	dir  string
	lock *os.File
}

func OpenStore(dir string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("state directory must be private (0700)")
	}
	lockPath := filepath.Join(dir, "instance.lock")
	if info, e := os.Lstat(lockPath); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("state lock must not be a symlink")
	}
	f, e := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("another beacon instance is using this state directory")
	}
	return &Store{dir, f}, nil
}
func (s *Store) Close() error { return s.lock.Close() }
func (s *Store) Read(out any) error {
	path := filepath.Join(s.dir, "state.json")
	info, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1<<20 {
		return errors.New("invalid state file")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return jsonx.Decode(raw, out)
}
func (s *Store) Write(v any) error {
	raw, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(s.dir, ".state-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if e = f.Chmod(0600); e != nil {
		return e
	}
	if _, e = f.Write(raw); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(f.Name(), filepath.Join(s.dir, "state.json")); e != nil {
		return e
	}
	d, e := os.Open(s.dir)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
