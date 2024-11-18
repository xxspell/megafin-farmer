package config

import (
	"encoding/json"
	"log"
	"os"
)

var GlobalConfig Config

type Config struct {
	Port    string `json:"port"`
	RefCode string `json:"ref_code"`
}

var defaultConfig = Config{
	Port:    "2112",
	RefCode: "97149c0c",
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
