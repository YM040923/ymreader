package store

// ============================================================
// WorkUnitTitle —— 章节标题回填结果
//
// 有些来源的漫画文件夹只有「第12话」这种编号，没有每话标题。
// 回填功能按作品匹配腾讯动漫的章节列表后，把标题单独存下来，
// 不修改任何磁盘文件；展示时拼在编号后面。
//
// 主键只用 unitId：Unit ID 已经包含 work/comic/path 信息，且合并、
// 拆分 Work 时保持不变，用 unitId 才能跟着单元走。
// ============================================================

func init() {
	Migrations = append(Migrations, Migration{
		Version:     52,
		Description: "Persist backfilled chapter titles for Work units",
		SQL: `CREATE TABLE IF NOT EXISTS "WorkUnitTitle" (
		        "unitId"    TEXT NOT NULL PRIMARY KEY,
		        "workId"    TEXT NOT NULL DEFAULT '',
		        "title"     TEXT NOT NULL DEFAULT '',
		        "source"    TEXT NOT NULL DEFAULT '',
		        "updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		      );
		      CREATE INDEX IF NOT EXISTS "WorkUnitTitle_workId_idx" ON "WorkUnitTitle"("workId");`,
	})
}

// WorkUnitTitle is one persisted chapter title override.
type WorkUnitTitle struct {
	UnitID string
	WorkID string
	Title  string
	Source string
}

// GetAllWorkUnitTitles returns unitId -> title for every persisted override.
// The table is small (only units that needed backfilling), so loading it in
// full keeps catalog assembly simple and merge/split safe.
func GetAllWorkUnitTitles() (map[string]string, error) {
	result := make(map[string]string)
	if db == nil || !workTableExists("WorkUnitTitle") {
		return result, nil
	}
	rows, err := db.Query(`SELECT "unitId", "title" FROM "WorkUnitTitle" WHERE "title" != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var unitID, title string
		if err := rows.Scan(&unitID, &title); err != nil {
			return nil, err
		}
		result[unitID] = title
	}
	return result, rows.Err()
}

// SaveWorkUnitTitles upserts a batch of chapter title overrides.
func SaveWorkUnitTitles(titles []WorkUnitTitle) error {
	if len(titles) == 0 || db == nil || !workTableExists("WorkUnitTitle") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO "WorkUnitTitle" ("unitId", "workId", "title", "source", "updatedAt")
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT("unitId") DO UPDATE SET
			"workId" = excluded."workId",
			"title" = excluded."title",
			"source" = excluded."source",
			"updatedAt" = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, title := range titles {
		if title.UnitID == "" || title.Title == "" {
			continue
		}
		if _, err := stmt.Exec(title.UnitID, title.WorkID, title.Title, title.Source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteWorkUnitTitlesByUnitIDs removes overrides for the given units.
func DeleteWorkUnitTitlesByUnitIDs(unitIDs []string) error {
	if len(unitIDs) == 0 || db == nil || !workTableExists("WorkUnitTitle") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`DELETE FROM "WorkUnitTitle" WHERE "unitId" = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, unitID := range unitIDs {
		if _, err := stmt.Exec(unitID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
