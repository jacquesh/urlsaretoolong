package main

import (
	"database/sql"
	"github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

func getDbConnection() *sql.DB {
	dbPath := "./urls.db"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(err)
	}

	// TODO: Are these pragmas even necessary? We *really* don't need a lot of speed here
	if _, err = os.Stat(dbPath); os.IsNotExist(err) {
		log.Printf("The database does not exist at %s, creating it...", dbPath)
		_, err = db.Exec("PRAGMA synchronous = NORMAL")
		_, err = db.Exec("PRAGMA journal_mode = WAL")
		_, err = db.Exec(`CREATE TABLE Urls (
							ShortUrl TEXT NOT NULL,
							LongUrl TEXT NOT NULL,
							CONSTRAINT UNQ_Urls_ShortUrl UNIQUE(ShortUrl)
						)`)
		if err != nil {
			panic(err)
		}
	}

	return db
}

type HttpRequestHandler struct{}

func (reqHandler *HttpRequestHandler) ServeHTTP(write http.ResponseWriter, req *http.Request) {
	log.Printf("[%s] src=%s dest=%s", req.Method, req.RemoteAddr, req.URL.Path)

	if req.Method == "POST" {
		db := getDbConnection()
		shortenUrl(write, req, db)
		err := db.Close()
		if err != nil {
			panic(err)
		}

	} else {
		if req.URL.Path == "/spectre.min.css" {
			http.ServeFile(write, req, "static/spectre.min.css")
		} else if req.URL.Path == "/" {
			http.ServeFile(write, req, "static/home.html")

		} else {
			db := getDbConnection()
			expandUrl(write, req, db)
			err := db.Close()
			if err != nil {
				panic(err)
			}
		}
	}
}

func expandUrl(write http.ResponseWriter, req *http.Request, db *sql.DB) {
	log.Printf("Expanding shortened URL: %s...\n", req.URL.Path)

	shortUrl := req.URL.Path[1:]
	rows, err := db.Query("SELECT LongUrl FROM Urls WHERE ShortUrl = ?1", shortUrl)
	if !rows.Next() {
		write.WriteHeader(http.StatusNotFound)
		return
	}

	var longUrl string
	err = rows.Scan(&longUrl)
	rows.Close()

	err = db.Close()
	if err != nil {
		panic(err)
	}

	http.Redirect(write, req, longUrl, http.StatusFound)
}

func shortenUrl(write http.ResponseWriter, req *http.Request, db *sql.DB) {
	err := req.ParseForm()
	if err != nil {
		panic(err)
	}

	longUrl, hasLongUrl := req.PostForm["url_longform"]
	if !hasLongUrl {
		write.WriteHeader(http.StatusBadRequest)
		return
	}

	desiredShortUrl, hasShortUrl := req.PostForm["url_desiredshortform"]
	if !hasShortUrl {
		write.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO Urls VALUES(?1, ?2)", desiredShortUrl[0], longUrl[0])
	if err != nil {
		sqliteErr := err.(sqlite3.Error)
		if sqliteErr.Code == 19 { // constraint failed
			// TODO:
			write.WriteHeader(http.StatusBadRequest)
			return
		} else {
			log.Printf("Encountered error %d: %s\n", sqliteErr.Code, sqliteErr.Code.Error())
			panic(err)
		}
	}

	log.Printf("Shorten %s to %s\n", longUrl, desiredShortUrl)
	http.ServeFile(write, req, "static/home.html")
}

func main() {
	log.Printf("USS v0.1. Compiled for %s, %s\n", runtime.GOOS, runtime.GOARCH)
	log.Print("Setting up web server...")

	handler := HttpRequestHandler{}
	server := http.Server{
		Addr:         "0.0.0.0:8081",
		Handler:      &handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Running web server at: http://%s ...", server.Addr)
	log.Fatal(server.ListenAndServe())
}
