package config

import (
	"encoding/json"
	"github.com/valyala/fasthttp"
	"log"
	"megafin_farmer/headers"
	"os"
	"time"
)

var GlobalConfig Config

type Config struct {
	Port            string `json:"port"`
	RefCode         string `json:"ref_code"`
	ApiKeyScrapeops string `json:"api_key_scrapeops"`
}

var defaultConfig = Config{
	Port:            "2112",
	RefCode:         "97149c0c",
	ApiKeyScrapeops: "c2d7efbb-817e-4957-9fc3-e5a7b083ab76", // Fake acc
}

var GlobalHeadersManager *headers.Manager

func InitHeadersManager(apiKey string) {
	httpClient := &fasthttp.Client{
		MaxConnsPerHost: 100,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
	}
	GlobalHeadersManager = headers.NewHeadersManager(apiKey, httpClient)
}

func loadConfig(filename string) Config {
	config := defaultConfig

	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Could not open config file: %v. Using default values", err)
		return config
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Printf("Could not decode config file: %v. Using default values", err)
		return defaultConfig
	}

	return config
}

func InitConfig(filename string) {

	GlobalConfig = loadConfig(filename)
}
