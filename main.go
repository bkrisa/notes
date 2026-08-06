package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	_ "modernc.org/sqlite"
	"tailscale.com/client/local"
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
		result, err := db.Exec("INSERT INTO notes (content) VALUES (?)", content)
		if err != nil {
			fmt.Println("Save error:", err)
			return
		}
		id, _ := result.LastInsertId()
		buffer = nil
		fmt.Println("--- Save note ---")

		serverURL := os.Getenv("QNOTE_SERVER")
		if serverURL != "" {
			row := db.QueryRow("SELECT created_at FROM notes WHERE id = ?", id)
			var createAt string
			row.Scan(&createAt)

			err := uploadNote(serverURL, int(id), content, createAt)
			if err != nil {
				// no internet or server unavailable - we move on silently
			}
		}
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

		id, err := strconv.Atoi(keyword)
		if err != nil {
			id = -1
		}

		rows, _ := db.Query("SELECT * FROM notes WHERE id = ? OR content LIKE ?", id, "%"+keyword+"%")
		defer rows.Close()
		for rows.Next() {
			var rowId int
			var content, createAt string
			rows.Scan(&rowId, &content, &createAt)
			fmt.Printf("------------\n[%d] %s\n%s\n------------\n", rowId, createAt, content)
		}
	}
	if strings.HasPrefix(command, ":date ") {
		dateQuery := strings.TrimPrefix(command, ":date ")

		rows, _ := db.Query("SELECT * FROM notes WHERE created_at LIKE ?", "%"+dateQuery+"%")
		defer rows.Close()
		for rows.Next() {
			var rowId int
			var content, createAt string
			rows.Scan(&rowId, &content, &createAt)
			fmt.Printf("------------\n[%d] %s\n%s\n------------\n", rowId, createAt, content)
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

			serverURL := os.Getenv("QNOTE_SERVER")
			if serverURL != "" {
				err := uploadNote(serverURL, id, newContent, createAt)
				if err != nil {
					// no internet or server unavailable - we move on silently
				}
			}
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

			serverURL := os.Getenv("QNOTE_SERVER")
			if serverURL != "" {
				err := deleteNoteOnServer(serverURL, id)
				if err != nil {
					// no internet or server unavailable - we move on silently
				}
			}
		}
	}
}

func runServer(db *sql.DB) {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "pong")
	})

	http.HandleFunc("/notes", requireTailscale(func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT id, content, created_at FROM notes ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type Note struct {
			ID       int    `json:"id"`
			Content  string `json:"content"`
			CreateAt string `json:"created_at"`
		}

		var notes []Note
		for rows.Next() {
			var n Note
			rows.Scan(&n.ID, &n.Content, &n.CreateAt)
			notes = append(notes, n)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(notes)
	}))

	http.HandleFunc("/notes/upload", requireTailscale(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type Note struct {
			ID       int    `json:"id"`
			Content  string `json:"content"`
			CreateAt string `json:"created_at"`
		}

		var n Note
		err := json.NewDecoder(r.Body).Decode(&n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err = db.Exec(`
			INSERT INTO notes (id, content, created_at) VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET content = excluded.content
		`, n.ID, n.Content, n.CreateAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(w, "OK")
	}))

	http.HandleFunc("/notes/delete", requireTailscale(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type DeleteRequest struct {
			ID int `json:"id"`
		}

		var req DeleteRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		db.Exec("DELETE FROM notes WHERE id = ?", req.ID)
		fmt.Fprintln(w, "OK")
	}))

	fmt.Println("Server listening on :8080")
	listenAddr := os.Getenv("QNOTE_SERVER")
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	err := http.ListenAndServe(listenAddr, nil)
	if err != nil {
		fmt.Println("Server error:", err)
	}
}

func syncFromServer(db *sql.DB, serverURL string) {
	resp, err := http.Get(serverURL + "/notes")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	type Note struct {
		ID       int    `json:"id"`
		Content  string `json:"content"`
		CreateAt string `json:"created_at"`
	}

	var notes []Note
	err = json.NewDecoder(resp.Body).Decode(&notes)
	if err != nil {
		fmt.Println("Decoder error:", err)
		return
	}

	for _, n := range notes {
		_, err := db.Exec(`
			INSERT INTO notes (id, content, created_at) VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET content = excluded.content
		`, n.ID, n.Content, n.CreateAt)
		if err != nil {
			fmt.Println("Insert/update error:", err)
		}
	}

	fmt.Printf("Synced %d notes.\n", len(notes))
}

func uploadNote(serverURL string, id int, content string, createAt string) error {
	type Note struct {
		ID       int    `json:"id"`
		Content  string `json:"content"`
		CreateAt string `json:"created_at"`
	}

	n := Note{ID: id, Content: content, CreateAt: createAt}
	body, _ := json.Marshal(n)

	resp, err := http.Post(serverURL+"/notes/upload", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Println("Uploaded note ID:", n.ID)

	return nil
}

func deleteNoteOnServer(serverURL string, id int) error {
	type DeleteRequest struct {
		ID int `json:"id"`
	}

	req := DeleteRequest{ID: id}
	body, _ := json.Marshal(req)

	resp, err := http.Post(serverURL+"/notes/delete", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func requireTailscale(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusForbidden)
			return
		}

		ip :=net.ParseIP(host)
		if ip == nil || ip.IsLoopback() {
			http.Error(w, "Unauthorized: loopback not allowed", http.StatusForbidden)
			return
		}

		var lc local.Client
		who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			http.Error(w, "Unauthorized: not a recognized Tailscale device", http.StatusForbidden)
			return
		}

		fmt.Println("Request from Tailscale device:", who.Node.Name)
		next(w, r)
	}
}

func main() {
	db := initDB()

	// run server
	if len(os.Args) > 1 && os.Args[1] == "--server" {
		runServer(db)
		return
	}

	serverURL := os.Getenv("QNOTE_SERVER")
	if serverURL != "" {
		syncFromServer(db, serverURL)
	}

	if len(os.Args) > 1 {
		// there is an extra argument, e.g. ":ls" or ":find egg"
		runCommand(db, strings.Join(os.Args[1:], " "))
		return
	}
	// no extra arguments -> normal note-taking mode
	scanner(db)
}
