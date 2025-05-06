package common

import (
	"os"
)

type FileSystemInterface interface {
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	UserHomeDir() (string, error)
	Remove(name string) error
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

type RealFileSystem struct{}

func (fs *RealFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }
func (fs *RealFileSystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) }
func (fs *RealFileSystem) UserHomeDir() (string, error)          { return os.UserHomeDir() }
func (fs *RealFileSystem) Remove(name string) error              { return os.Remove(name) }
func (fs *RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *RealFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}
