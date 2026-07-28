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

func runCommand(db *sql.DB, command string) {
	if command == ":ls" {
		rows, _ := db.Query("SELECT id, content, created_at FROM notes ORDER BY created_at DESC")
		defer rows.Close()
		for rows.Next() {
			var id int
			var content, createAt string
			rows.Scan(&id, &content, &createAt)
			fmt.Printf("[%d] %s\n%s\n------------\n", id, createAt, content)
		}
	}
	if strings.HasPrefix(command, ":find ") {
		keyword := strings.TrimPrefix(command, ":find ")
		rows, _ := db.Query("SELECT id, content, created_at FROM notes WHERE content LIKE ?", "%"+keyword+"%")
		defer rows.Close()
		for rows.Next() {
			var id int
			var content, createAt string
			rows.Scan(&id, &content, &createAt)
			fmt.Printf("------------\n[%d] %s\n%s\n------------\n", id, createAt, content)
		}
	}
}

func main() {
	db := initDB()

	if len(os.Args) > 1 {
		// there is an extra argument, e.g. ":ls" or ":find egg"
		command := strings.Join(os.Args[1:], " ")
		runCommand(db, command)
		return
	}

	// no extra arguments -> normal note-taking mode
	scanner(db)
}
