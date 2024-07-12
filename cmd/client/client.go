package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type Quote struct {
	Bid float64 `json:"bid"`
}

func getQuote() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://localhost:8080/cotacao",
		nil,
	)
	if err != nil {
		return 0.0, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0.0, err
	}
	defer res.Body.Close()

	var quote Quote
	err = json.NewDecoder(res.Body).Decode(&quote)
	if err != nil {
		return 0.0, err
	}

	return quote.Bid, nil
}

func saveQuoteToFile(dollar float64) error {
	file, err := os.Create("cotacao.txt")
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("Dólar: %.4f", dollar))
	if err != nil {
		return err
	}

	return nil
}

func main() {
	log.Println("Iniciando a aplicação...")

	log.Println("Obtendo cotação...")
	dollar, err := getQuote()
	if err != nil {
		log.Fatalf("Erro ao obter cotação: %v", err)
	}

	log.Println("Salvando cotação...")
	err = saveQuoteToFile(dollar)
	if err != nil {
		log.Fatalf("Erro ao salvar cotação: %v", err)
	}
}
