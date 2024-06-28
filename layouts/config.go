package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	PublicFolder string         `json:"public_folder"`
	Schedule     int            `json:"schedule"`
	Defaults     DefaultsConfig `json:"defaults"`
	WriteTo      string         `json:"write_to"`
	Rules        []RuleConfig   `json:"rules"`
	Tests        TestsConfig    `json:"tests"`
	Protocol     string         `json:"protocol"`
}

type DefaultsConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Api        string `json:"api"`
	TimeWindow string `json:"time_window"`
	Step       string `json:"step"`
	Bucket     string `json:"bucket"`
}

type RuleConfig struct {
	Name      string   `json:"name"`
	Condition string   `json:"condition"`
	Queries   []string `json:"queries"`
}

type TestsConfig struct {
	Mode string `json:"mode"`
}

func LoadConfig(path string) (*Config, error) {
	configFile, err := os.Open(path)
	if err != nil {
		return &Config{}, fmt.Errorf("error opening config file: %w", err)
	}
	defer configFile.Close()

	var config Config
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&config); err != nil {
		return &Config{}, fmt.Errorf("errro parsing config file: %w", err)
	}

	return &config, nil
}
