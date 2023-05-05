package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/tyler-smith/go-bip39"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"log"
	"math/big"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	checkInterval = 20 * time.Millisecond
	dbConnection  = "postgres://postgres:12345@localhost:5432/postgres?sslmode=disable"

	numWorkers = 30
)

func main() {
	runtime.GOMAXPROCS(8)
	//client := liteclient.NewConnectionPool()
	//
	//configUrl := "https://ton-blockchain.github.io/global.config.json"
	//err := client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	//if err != nil {
	//	panic(err)
	//}
	//api := ton.NewAPIClient(client)

	db, err := sql.Open("postgres", dbConnection)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS balances (seed TEXT NOT NULL, balance TEXT NOT NULL);`)
	if err != nil {
		log.Fatal(err)
	}

	seedsChan := make(chan string, numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		go worker(seedsChan, &wg, db)
	}

	for {
		mySeed := generateRandomSeedPhrase()
		wg.Add(1)
		seedsChan <- mySeed
		wg.Wait()
		time.Sleep(checkInterval)
	}
}

func worker(seedsChan chan string, wg *sync.WaitGroup, db *sql.DB) {
	for seed := range seedsChan {
		log.Println(seed)
		// Получение баланса для seed-фразы
		// Замените эту функцию на вашу реализацию для проверки баланса
		balance, err := checkBalance(seed)
		if err != nil {
			log.Println("Ошибка при получении баланса:", err)
			wg.Done()
			continue
		}
		{
			// Запись баланса и seed-фразы в базу данных
			_, err = db.Exec(`INSERT INTO balances (seed, balance) VALUES ($1, $2)`, seed, balance)
			if err != nil {
				log.Println("Ошибка при записи баланса в базу данных:", err)
			}
		}
		wg.Done()

	}
}

func generateRandomSeedPhrase() string {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		log.Fatal(err)
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		log.Fatal(err)
	}

	return mnemonic
}

func checkBalance(seed string) (string, error) {
	client := liteclient.NewConnectionPool()

	bigInt := new(big.Int)
	_, okay := bigInt.SetString("0", 10)

	if !okay {
		log.Println("Не удалось конвертировать строку в *big.Int")
	}

	configUrl := "https://ton-blockchain.github.io/global.config.json"
	err := client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		time.Sleep(1 * time.Second)
	}
	api := ton.NewAPIClient(client)

	words := strings.Split(seed, " ")
	fmt.Println(words)
	w, err := wallet.FromSeed(api, words, wallet.V3)
	if err != nil {
		log.Println("неверный сид")
		return "nil", nil
	}

	block, err := api.CurrentMasterchainInfo(context.Background())
	if err != nil {
		time.Sleep(1 * time.Second)
	}

	balance, err := w.GetBalance(context.Background(), block)
	if err != nil {
		log.Println("не удалось получить баланс")
		return "nil", nil
	}

	return balance.TON(), nil

}
