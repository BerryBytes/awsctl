package common

import (
	"os"
)

type FileSystemInterface interface {
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	UserHomeDir() (string, error)
}

type RealFileSystem struct{}

func (fs *RealFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }
func (fs *RealFileSystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) }
func (fs *RealFileSystem) UserHomeDir() (string, error)          { return os.UserHomeDir() }
