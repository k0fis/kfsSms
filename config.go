package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Sms    SmsConfig    `yaml:"sms"`
	Api    ApiConfig    `yaml:"api"`
	Msisdn MsisdnConfig `yaml:"msisdn"`
	Update UpdateConfig `yaml:"update"`
}

type SmsConfig struct {
	PortName             string `yaml:"portName"`
	BaudRate             int    `yaml:"baudRate"`
	PollIntervalMs       int    `yaml:"pollIntervalMs"`
	OutgoingPollMs       int    `yaml:"outgoingPollIntervalMs"`
	OpenModem            bool   `yaml:"openModem"`
	SendMaxRetries       int    `yaml:"sendMaxRetries"`
	SendRetryDelayMs     int    `yaml:"sendRetryDelayMs"`
	PoolRetryCount       int    `yaml:"poolRetryCount"`
	PoolRetryDelayMs     int    `yaml:"poolRetryDelay"`
}

type ApiConfig struct {
	BaseUrl  string `yaml:"baseUrl"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type MsisdnConfig struct {
	Pin string `yaml:"pin"`
}

type UpdateConfig struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
}

func loadConfig(path string) (*Config, error) {
	cfg := &Config{
		Sms: SmsConfig{
			PortName:         "COM3",
			BaudRate:         115200,
			PollIntervalMs:   5000,
			OutgoingPollMs:   5000,
			OpenModem:        true,
			SendMaxRetries:   3,
			SendRetryDelayMs: 5000,
			PoolRetryCount:   3,
			PoolRetryDelayMs: 5000,
		},
		Update: UpdateConfig{
			Owner: "k0fis",
			Repo:  "kfsSms",
		},
	}

	if path == "" {
		path = "config.yml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	if cfg.Api.BaseUrl == "" {
		return nil, fmt.Errorf("api.baseUrl is required")
	}
	if cfg.Api.User == "" {
		return nil, fmt.Errorf("api.user is required")
	}

	return cfg, nil
}
