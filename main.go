package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

func limitMiddleware(next http.Handler) http.Handler {
	return next
}

func gatewayHandler(target *url.URL) http.Handler {
	return httputil.NewSingleHostReverseProxy(target)
}

type Route struct {
	Name    string `mapstructure:"name"`
	Context string `mapstructure:"context"`
	Target  string `mapstructure:"target"`
}

type GatewayConfig struct {
	ListenAddr string  `mapstructure:"listenAddr"`
	Routes     []Route `mapstructure:"routes"`
}

func main() {
	viper.AddConfigPath("./config")
	viper.SetConfigType("yaml")
	viper.SetConfigName("default")
	err := viper.ReadInConfig()
	if err != nil {
		log.Println("Warning could not load configuration")
	}

	viper.AutomaticEnv()

	gatewayConfig := &GatewayConfig{}
	err = viper.UnmarshalKey("gateway", gatewayConfig)
	if err != nil {
		panic(err)
	}

	gmux := mux.NewRouter()

	for _, route := range gatewayConfig.Routes {
		backendURL, err := url.Parse(route.Target)
		if err != nil {
			log.Fatal("Error parsing backend URL:", err)
		}

		gmux.PathPrefix("/").Handler(gatewayHandler(backendURL))
	}

	log.Printf("Starting smart reverse proxy on [%s]", gatewayConfig.ListenAddr)
	if err := http.ListenAndServe(gatewayConfig.ListenAddr, limitMiddleware(gmux)); err != nil {
		log.Fatalf("Unable to start server: %s", err.Error())
	}
	return
}
