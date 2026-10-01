package storage

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Storage struct {
	UploadDir string
}

func New(uploadDir string) (*Storage, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}
	return &Storage{UploadDir: uploadDir}, nil
}

func (s *Storage) SaveFile(file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
		".gif":  true,
	}

	if !allowed[ext] {
		return "", errors.New("invalid file type: only jpg, jpeg, png, webp, and gif allowed")
	}

	newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	targetPath := filepath.Join(s.UploadDir, newFilename)

	dst, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create target file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return "/uploads/" + newFilename, nil
}
