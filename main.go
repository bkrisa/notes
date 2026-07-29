package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strconv"
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
	
		if row == ":u" {
			if len(buffer) > 0 {
				buffer = buffer[:len(buffer)-1]
				fmt.Println("Last line deleted.")
			}
		}
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
	if strings.HasPrefix(command, ":edit ") {
		idStr := strings.TrimPrefix(command, ":edit ")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			fmt.Println("Invalid: ID:", idStr)
			return
		}

		noteRow := db.QueryRow("SELECT id, content, created_at FROM notes WHERE id = ?", id)
		var content, createAt string
		err = noteRow.Scan(&id, &content, &createAt)
		if err != nil {
			fmt.Println("No such note:", id)
			return
		}
		fmt.Printf("[%d] %s\n", id, createAt)

		oldLines := strings.Split(content, "\n")
		editScanner := bufio.NewScanner(os.Stdin)
		var newBuffer []string

		fmt.Println("--- Edit note (Enter = remains, or rewrite the line) ---")
		for _, oldLine := range oldLines {
			fmt.Printf("(%s) > ", oldLine)
			editScanner.Scan()
			input := editScanner.Text()

			if input == "" {
				newBuffer = append(newBuffer, oldLine)
				continue
			} else {
				newBuffer = append(newBuffer, input)
			}
		}

		save := func() {
			newContent := strings.Join(newBuffer, "\n")
			db.Exec("UPDATE notes SET content = ? WHERE id = ?", newContent, id)
			newBuffer = nil
			fmt.Println("--- Note updated ---")
		}
		
		for {
			fmt.Print("note> ")
			editScanner.Scan()
			row := editScanner.Text()
		
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
			
			newBuffer = append(newBuffer, row)
		}
	}
	if strings.HasPrefix(command, ":d ") {
		idStr := strings.TrimPrefix(command, ":d ")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			fmt.Println("Invalid: ID:", idStr)
			return
		}

		result, err := db.Exec("DELETE FROM notes WHERE id = ?", id)
		if err != nil {
			fmt.Println("Delete error:", err)
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			fmt.Printf("No such note: %d\n", id)
		} else {
			fmt.Printf("--- Deleted note [%d] ---\n", id)
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
