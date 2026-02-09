package fs

import (
	"os"
)

func fileExistsAt(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
