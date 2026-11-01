package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func init() {
	Migrations = append(Migrations, Migration{
		Version:     50,
		Description: "Record manual Work operations for operation-level undo",
		SQL: `CREATE TABLE IF NOT EXISTS "WorkOperation" (
		        "id" TEXT NOT NULL PRIMARY KEY,
		        "operationType" TEXT NOT NULL,
		        "targetWorkId" TEXT NOT NULL,
		        "affectedWorkIds" TEXT NOT NULL DEFAULT '[]',
		        "snapshot" TEXT NOT NULL DEFAULT '[]',
		        "createdBy" TEXT NOT NULL DEFAULT '',
		        "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		      );
		      CREATE INDEX IF NOT EXISTS "WorkOperation_createdAt_idx" ON "WorkOperation"("createdAt" DESC);`,
	})
}

type WorkMembershipSnapshot struct {
	ComicID   string `json:"comicId"`
	WorkID    string `json:"workId,omitempty"`
	LibraryID string `json:"libraryId"`
}

type WorkOperation struct {
	ID              string
	OperationType   string
	TargetWorkID    string
	AffectedWorkIDs []string
	Snapshot        []WorkMembershipSnapshot
	CreatedBy       string
	CreatedAt       time.Time
}

func RecordWorkOperation(operation WorkOperation) error {
	if db == nil || !workTableExists("WorkOperation") {
		return nil
	}
	affected, err := json.Marshal(operation.AffectedWorkIDs)
	if err != nil {
		return err
	}
	snapshot, err := json.Marshal(operation.Snapshot)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO "WorkOperation" (
			"id", "operationType", "targetWorkId", "affectedWorkIds", "snapshot", "createdBy", "createdAt"
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, operation.ID, operation.OperationType, operation.TargetWorkID, string(affected), string(snapshot), operation.CreatedBy)
	return err
}

// LatestWorkOperation returns the newest operation that touched any of the
// supplied Work IDs.
func LatestWorkOperation(workIDs []string) (*WorkOperation, error) {
	if db == nil || !workTableExists("WorkOperation") {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT "id", "operationType", "targetWorkId", "affectedWorkIds", "snapshot", "createdBy", "createdAt"
		FROM "WorkOperation"
		ORDER BY rowid DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	wanted := make(map[string]struct{}, len(workIDs))
	for _, workID := range workIDs {
		wanted[workID] = struct{}{}
	}
	for rows.Next() {
		var operation WorkOperation
		var affectedJSON, snapshotJSON string
		if err := rows.Scan(
			&operation.ID, &operation.OperationType, &operation.TargetWorkID,
			&affectedJSON, &snapshotJSON, &operation.CreatedBy, &operation.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(affectedJSON), &operation.AffectedWorkIDs); err != nil {
			return nil, fmt.Errorf("decode affected Work IDs: %w", err)
		}
		if err := json.Unmarshal([]byte(snapshotJSON), &operation.Snapshot); err != nil {
			return nil, fmt.Errorf("decode Work snapshot: %w", err)
		}
		for _, workID := range operation.AffectedWorkIDs {
			if _, ok := wanted[workID]; ok {
				return &operation, nil
			}
		}
	}
	return nil, rows.Err()
}

// UndoWorkOperation restores membership rows captured before an operation and
// removes the operation record in one transaction.
func UndoWorkOperation(operationID string, snapshot []WorkMembershipSnapshot) error {
	if db == nil || !workTableExists("WorkOperation") {
		return nil
	}
	logicalWorkWriteMu.Lock()
	defer logicalWorkWriteMu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(snapshot) > 0 {
		comicIDs := make([]string, 0, len(snapshot))
		for _, entry := range snapshot {
			if comicID := strings.TrimSpace(entry.ComicID); comicID != "" {
				comicIDs = append(comicIDs, comicID)
			}
		}
		if len(comicIDs) > 0 {
			placeholders := strings.TrimRight(strings.Repeat("?,", len(comicIDs)), ",")
			args := make([]interface{}, 0, len(comicIDs))
			for _, comicID := range comicIDs {
				args = append(args, comicID)
			}
			if _, err := tx.Exec(`DELETE FROM "WorkMembership" WHERE "comicId" IN (`+placeholders+`)`, args...); err != nil {
				return err
			}
		}
		for _, entry := range snapshot {
			if strings.TrimSpace(entry.ComicID) == "" || strings.TrimSpace(entry.WorkID) == "" ||
				strings.TrimSpace(entry.LibraryID) == "" {
				continue
			}
			if _, err := tx.Exec(`
				INSERT INTO "WorkMembership" ("comicId", "workId", "libraryId", "createdAt", "updatedAt")
				VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				ON CONFLICT("comicId") DO UPDATE SET
					"workId" = excluded."workId",
					"libraryId" = excluded."libraryId",
					"updatedAt" = CURRENT_TIMESTAMP
			`, entry.ComicID, entry.WorkID, entry.LibraryID); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`DELETE FROM "WorkOperation" WHERE "id" = ?`, operationID); err != nil {
		return err
	}
	return tx.Commit()
}
