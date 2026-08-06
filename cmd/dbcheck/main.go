package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := "data/nowen-reader.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	fmt.Printf("=== SQLite Index Repair Tool ===\n")
	fmt.Printf("Database: %s\n\n", dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// REINDEX 重建所有索引，修复 "row missing from index" 问题
	fmt.Println("[1] Running REINDEX to rebuild all indexes...")
	if _, err := db.Exec("REINDEX"); err != nil {
		log.Fatalf("REINDEX failed: %v", err)
	}
	fmt.Println("  ✅ REINDEX completed")

	// 再次检查完整性
	fmt.Println("\n[2] Running integrity_check after repair...")
	rows, err := db.Query("PRAGMA integrity_check")
	if err != nil {
		log.Fatalf("integrity_check failed: %v", err)
	}
	allOk := true
	for rows.Next() {
		var result string
		rows.Scan(&result)
		fmt.Printf("  %s\n", result)
		if result != "ok" {
			allOk = false
		}
	}
	rows.Close()

	if allOk {
		fmt.Println("\n🎉 Database repaired successfully! All indexes are healthy.")
	} else {
		fmt.Println("\n⚠️  Some issues remain after REINDEX. Consider using VACUUM to rebuild the entire database.")
		fmt.Println("  Attempting VACUUM...")
		if _, err := db.Exec("VACUUM"); err != nil {
			fmt.Printf("  ❌ VACUUM failed: %v\n", err)
			fmt.Println("  You need to stop the server first, then run this tool again.")
		} else {
			fmt.Println("  ✅ VACUUM completed")
		}
	}
}
