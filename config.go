package main

type TLSConfig struct {
	Enabled        bool   `yaml:"enabled"`
	ClientKeyFile  string `yaml:"key"`
	ClientCertFile string `yaml:"cert"`
	CaFile         string `yaml:"ca"`
}

type Config struct {
	Port     string    `yaml:"port"`
	Registry string    `yaml:"registry"`
	TLS      TLSConfig `yaml:"tls"`
	Scheme   string
	Host     string
}
