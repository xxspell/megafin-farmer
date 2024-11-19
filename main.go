package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"megafin_farmer/config"
	"megafin_farmer/core"
	"megafin_farmer/metrics"
	"megafin_farmer/utils"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

func inputUser() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	return strings.TrimSpace(scanner.Text())
}

func handlePanic() {
	if r := recover(); r != nil {
		log.Printf("Unexpected Error: %v", r)
		fmt.Println("Press Enter to Exit..")
		_, err := fmt.Scanln()
		if err != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func startTasks(userAction int,
	accountsList []string,
	proxyList []string) {

	var wg sync.WaitGroup
	metrics.ActiveAccounts.Set(float64(len(accountsList)))

	if userAction == 1 {
		fmt.Println()
		for i := 0; i < len(accountsList); i++ {
			account := accountsList[i]
			proxy := proxyList[i]

			wg.Add(1)

			go func(acc, prox string) {
				defer wg.Done()

				core.StartFarmAccount(acc, prox)
			}(account, proxy)
		}

		wg.Wait()

	} else if userAction == 2 {
		var generatedAccountsList []string
		fmt.Printf("\nHow Many Accounts Do You Want To Generate: ")
		accountsCountToGenerate, err := strconv.Atoi(inputUser())
		fmt.Println()

		if err != nil {
			log.Panicf("Error converting account count to int: %v", err)
		}

		for i := 0; i < accountsCountToGenerate; i++ {
			privateKey, err := crypto.GenerateKey()
			if err != nil {
				log.Printf("Error generating private key: %v", err)
				continue
			}

			privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))

			log.Printf("Successfully Generated Account %s | [%d/%d]", privateKeyHex, i+1, accountsCountToGenerate)

			generatedAccountsList = append(generatedAccountsList, privateKeyHex)
		}

		utils.AppendFile("./data/accounts.txt", strings.Join(generatedAccountsList, "\n")+"\n")

	} else if userAction == 3 {
		var totalMgfBalance, totalUsdcBalance float64
		var mu sync.Mutex

		fmt.Println()
		for i := 0; i < len(accountsList); i++ {
			account := accountsList[i]
			proxy := proxyList[i]

			wg.Add(1)

			go func(acc, prox string) {
				defer wg.Done()

				mgfBalance, usdcBalance := core.ParseAccountBalance(acc, prox)

				mu.Lock()
				totalMgfBalance += mgfBalance
				totalUsdcBalance += usdcBalance
				mu.Unlock()
			}(account, proxy)
		}

		wg.Wait()

		fmt.Println()
		fmt.Printf("Total MGF Balance: %f\n", totalMgfBalance)
		fmt.Printf("Total USDC Balance: %f\n", totalUsdcBalance)

	}
}

func main() {
	config.InitConfig("config.json")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":"+config.GlobalConfig.Port, nil)
	}()

	var proxyListSorted []string
	var accountsListSorted []string

	config.InitHeadersManager(config.GlobalConfig.ApiKeyScrapeops)

	defer handlePanic()

	accountsList, err := utils.ReadFileByRows("./data/accounts.txt")

	if err != nil {
		log.Panicf("Error While Reading Accounts File: %v", err)
	}

	err1 := config.GlobalHeadersManager.PrepareHeadersForAccounts(len(accountsList))
	if err1 != nil {
		log.Fatalf("Failed to prepare headers: %v", err)
	}

	proxyList, err := utils.ReadFileByRows("./data/proxies.txt")

	if err != nil {
		log.Panicf("Error While Reading Proxy File: %v", err)
	}

	for _, proxy := range proxyList {
		parsedProxy, err := utils.ParseProxy(proxy)

		if err != nil {
			log.Printf("%s wrong proxy format", proxy)
			continue
		}

		proxyListSorted = append(proxyListSorted, parsedProxy)
	}

	for _, account := range accountsList {
		parsedAccountKey := utils.RemoveHexPrefix(account)
		accountsListSorted = append(accountsListSorted, parsedAccountKey)
	}

	fmt.Printf("Successfully Loaded %d Accounts // %d Proxies", len(accountsList), len(proxyListSorted))

	limit := len(accountsListSorted)

	if len(proxyListSorted) < limit {
		limit = len(proxyListSorted)
	}

	accountsListSorted = accountsListSorted[:limit]
	proxyListSorted = proxyListSorted[:limit]

	fmt.Printf("\n1. Start Farming" +
		"\n2. Generate Accounts" +
		"\n3. Parse Accounts Balance" +
		"\nEnter Your Action: ")

	userAction, err := strconv.Atoi(inputUser())

	if err != nil {
		log.Panicf("Error When Enter Your Action: %v", err)
	}

	startTasks(userAction, accountsListSorted, proxyListSorted)

	fmt.Printf("The Work Has Beeen Successfully Finished")
	fmt.Printf("\n\nPress Enter to Exit..")
	inputUser()
}
