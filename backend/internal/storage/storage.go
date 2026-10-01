// Package storage keeps uploaded item images on local disk. File names are
// random UUIDs; the app compresses to ≤1600 px JPEG before upload.
package storage

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/yasinatby/swappy/backend/internal/apperr"
)

const MaxImageBytes = 5 << 20

var (
	ErrNotJPEG  = apperr.New(apperr.Invalid, "invalid_image", "image must be a JPEG")
	ErrTooLarge = apperr.New(apperr.Invalid, "image_too_large", "image must be at most 5 MB")
	namePattern = regexp.MustCompile(`^[0-9a-f]{32}\.jpg$`)
)

type Local struct {
	Dir string
}

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Local{Dir: dir}, nil
}

// SaveJPEG stores the image and returns its name.
func (l *Local) SaveJPEG(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxImageBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxImageBytes {
		return "", ErrTooLarge
	}
	if !bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "", ErrNotJPEG
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%x.jpg", random)
	if err := os.WriteFile(filepath.Join(l.Dir, name), data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// Path resolves a stored name to a file path, rejecting anything that is not
// a name this package generated.
func (l *Local) Path(name string) (string, bool) {
	if !namePattern.MatchString(name) {
		return "", false
	}
	return filepath.Join(l.Dir, name), true
}

func (l *Local) Delete(name string) error {
	path, ok := l.Path(name)
	if !ok {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
