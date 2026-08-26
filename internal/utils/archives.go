package utils

import (
	"errors"
	"io"
	"os"
)

func ReadOrCreateFile(filePath string) ([]byte, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)

	if err == nil {
		file.Close()
		return nil, nil
	}

	if errors.Is(err, os.ErrExist) {
		existingFile, err := os.Open(filePath)

		if err != nil {
			return nil, err
		}

		defer existingFile.Close()

		content, err := io.ReadAll(existingFile)

		if err != nil {
			return nil, err
		}

		return content, nil
	}

	return nil, err
}