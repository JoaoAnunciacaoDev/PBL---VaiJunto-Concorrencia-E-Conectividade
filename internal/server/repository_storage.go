package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func readJSONFile(path string, destination any) error {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || len(content) == 0 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ler arquivo: %w", err)
	}

	if err := json.Unmarshal(content, destination); err != nil {
		return fmt.Errorf("interpretar JSON: %w", err)
	}

	return nil
}

func writeJSONFileAtomic(path, temporaryPrefix string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar JSON: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("criar diretório de dados: %w", err)
	}

	temporaryFile, err := os.CreateTemp(directory, temporaryPrefix)
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}

	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporaryFile.Write(data); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("gravar arquivo temporário: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("fechar arquivo temporário: %w", err)
	}

	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("substituir arquivo de dados: %w", err)
	}

	return nil
}
