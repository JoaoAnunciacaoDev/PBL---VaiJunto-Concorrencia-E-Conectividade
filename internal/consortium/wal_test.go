package consortium

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWALAppendAndReadAll(t *testing.T) {
	tempDir := t.TempDir()
	walPath := filepath.Join(tempDir, "wal.log")

	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("erro ao criar WAL: %v", err)
	}
	defer wal.Close()

	entry1 := WALEntry{
		TxID:          "tx-1",
		Role:          "participant",
		Status:        TxPrepared,
		CoordinatorID: "empresa-a",
		SegmentIDs:    []string{"seg-1"},
		UpdatedAt:     time.Now().Truncate(time.Millisecond),
	}
	if err := wal.Append(entry1); err != nil {
		t.Fatalf("erro ao fazer append entry1: %v", err)
	}

	entry2 := WALEntry{
		TxID:          "tx-1",
		Role:          "participant",
		Status:        TxCommitted,
		CoordinatorID: "empresa-a",
		SegmentIDs:    []string{"seg-1"},
		UpdatedAt:     time.Now().Truncate(time.Millisecond),
	}
	if err := wal.Append(entry2); err != nil {
		t.Fatalf("erro ao fazer append entry2: %v", err)
	}

	entries, err := wal.ReadAll()
	if err != nil {
		t.Fatalf("erro ao ler entradas do WAL: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("esperado 2 entradas no WAL, obtido %d", len(entries))
	}

	if entries[0].TxID != "tx-1" || entries[0].Status != TxPrepared {
		t.Fatalf("entrada 0 incorreta: %+v", entries[0])
	}
	if entries[1].TxID != "tx-1" || entries[1].Status != TxCommitted {
		t.Fatalf("entrada 1 incorreta: %+v", entries[1])
	}
}

func TestWALReopen(t *testing.T) {
	tempDir := t.TempDir()
	walPath := filepath.Join(tempDir, "wal.log")

	wal1, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("erro ao criar WAL: %v", err)
	}

	entry := WALEntry{
		TxID:          "tx-recovery",
		Role:          "coordinator",
		Status:        TxCommitted,
		CoordinatorID: "empresa-b",
		SegmentIDs:    []string{"seg-1", "seg-2"},
	}
	if err := wal1.Append(entry); err != nil {
		t.Fatalf("erro ao salvar entrada: %v", err)
	}
	wal1.Close()

	// Reabre o WAL após queda simulada
	wal2, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("erro ao reabrir WAL: %v", err)
	}
	defer wal2.Close()

	entries, err := wal2.ReadAll()
	if err != nil {
		t.Fatalf("erro ao ler WAL reaberto: %v", err)
	}

	if len(entries) != 1 || entries[0].TxID != "tx-recovery" || entries[0].Status != TxCommitted {
		t.Fatalf("recuperação após reabertura falhou: %+v", entries)
	}
}
