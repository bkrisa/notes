package main

import (
	"bufio"
	"fmt"
	"os"
	"database/sql"
	"strings"
	_ "modernc.org/sqlite"
)

func initDB() *sql.DB {
	db, err := sql.Open("sqlite", "./notes.db")
	if err != nil {
		fmt.Println("DB error:", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	db.Exec(createTable)

	return db
}

func scanner(db *sql.DB) {
	scanner := bufio.NewScanner(os.Stdin)
	var buffer []string
	save := func() {
		content := strings.Join(buffer, "\n")
		db.Exec("INSERT INTO notes (content) VALUES (?)", content)
		buffer = nil
		fmt.Println("--- Save note ---")
	}
	for {
		fmt.Print("note> ")
		scanner.Scan()
		row := scanner.Text()
		if row == ":q" {
			break
		}
		if row == ":w" {
			save()
			continue
		}
		if row == ":wq" {
			save()
			break
		}
		buffer = append(buffer, row)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading input:", err)
	}
}

func main() {
	db := initDB()
	scanner(db)
}
