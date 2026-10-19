package consortium

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TxStatus string

const (
	TxPrepared  TxStatus = "PREPARED"  // participante: assento pré-reservado sob lock
	TxCommitted TxStatus = "COMMITTED" // transação confirmada definitivamente
	TxAborted   TxStatus = "ABORTED"   // transação abortada/cancelada
	TxExpired   TxStatus = "EXPIRED"   // lock expirou por tempo limite (TTL)
)

// WALEntry representa uma entrada gravada no log de transações.
type WALEntry struct {
	TxID          string    `json:"tx_id"`
	Role          string    `json:"role"` // coordinator ou participant
	Status        TxStatus  `json:"status"`
	CoordinatorID string    `json:"coordinator_id"`
	SegmentIDs    []string  `json:"segment_ids,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WAL implementa um Write-Ahead Log seguro e persistente.
type WAL struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// NewWAL inicializa ou reabre o arquivo WAL no caminho especificado.
func NewWAL(path string) (*WAL, error) {
	if path == "" {
		return nil, errors.New("caminho do WAL não pode ser vazio")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("criar diretório do WAL: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("abrir arquivo do WAL: %w", err)
	}

	return &WAL{
		path: path,
		file: file,
	}, nil
}

// Append adiciona uma nova entrada ao fim do log de forma síncrona (fsync).
func (w *WAL) Append(entry WALEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if entry.TxID == "" {
		return errors.New("tx_id obrigatório no WAL")
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("serializar entrada do WAL: %w", err)
	}

	data = append(data, '\n')
	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("escrever no WAL: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sincronizar WAL em disco: %w", err)
	}

	return nil
}

// ReadAll lê todas as entradas históricas do WAL a partir do início do arquivo.
func (w *WAL) ReadAll() ([]WALEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("abrir WAL para leitura: %w", err)
	}
	defer file.Close()

	var entries []WALEntry
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry WALEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("linha %d inválida no WAL: %w", lineNumber, err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("ler WAL: %w", err)
	}

	return entries, nil
}

// Close fecha o descritor de arquivo aberto.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}
