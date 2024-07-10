package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/valyala/fastjson"
)

type Quote struct {
	Bid float64 `json:"bid"`
}

func getQuote() (*Quote, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://economia.awesomeapi.com.br/json/last/USD-BRL",
		nil,
	)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var p fastjson.Parser
	v, err := p.ParseBytes(bytes)
	if err != nil {
		return nil, err
	}

	bidStrB := v.GetObject("USDBRL").Get("bid").GetStringBytes()
	bid, err := strconv.ParseFloat(string(bidStrB), 64)
	if err != nil {
		return nil, err
	}

	return &Quote{
		Bid: bid,
	}, nil
}

func connectDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./server.db")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlStmt := `
	create table if not exists quotes (bid real not null, created_at datetime default current_timestamp);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	return db
}

func saveQuoteToDB(db *sql.DB, quote *Quote) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := db.ExecContext(ctx, "insert into quotes (bid) values ($1)", quote.Bid)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	db := connectDB()
	defer db.Close()

	server := &http.Server{
		Addr: ":8080",
	}

	mux := http.NewServeMux()
	server.Handler = mux

	mux.HandleFunc("/cotacao", func(w http.ResponseWriter, r *http.Request) {
		quote, err := getQuote()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Failed to get quote: %v", err)
			return
		}

		if err := saveQuoteToDB(db, quote); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Failed to save quote: %v", err)
			return
		}

		resp, err := json.Marshal(quote)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Failed to marshal quote: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Shutting down server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}

	log.Println("Server stopped")
}
