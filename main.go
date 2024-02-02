package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

type redisType struct {
	Pool *redis.Pool
}

var (
	redisClient *redisType
	once        sync.Once
)

const (
	// RedisIP is a IP where redis server is hosted
	RedisIP = "127.0.0.1"
)

const (
	// BUCKET time bucket for Key Generation.
	// We will append the minute value to the original key to create time bucket for Keys
	BUCKET = 1 * 60
	// EXPIRY is a time after which keys should expire in redis
	EXPIRY = 5 * 60
	// THRESHOLD is a rate limiting threshold after which e=we should fail the request
	THRESHOLD = 10
)

func GetRedisConn() redis.Conn {
	once.Do(func() {
		redisPool := &redis.Pool{
			MaxActive: 100,
			Dial: func() (redis.Conn, error) {
				rc, err := redis.Dial("tcp", RedisIP+":6379")
				if err != nil {
					fmt.Println("Error connecting to redis:", err.Error())
					return nil, err
				}
				return rc, nil
			},
		}
		redisClient = &redisType{
			Pool: redisPool,
		}
	})
	return redisClient.Pool.Get()
}

// GetKey returns a Key to be stored in Redis.
// It appends the minute value of Unix time stamp to create buckets for Key
func GetKey(IP string) string {
	bucket := time.Now().Unix() / BUCKET
	IP = IP + strconv.FormatInt(bucket, 10)
	return IP
}

func limitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn := GetRedisConn()
		defer conn.Close()
		IPAddress := r.Header.Get("X-Real-Ip")
		if IPAddress == "" {
			IPAddress = r.Header.Get("X-Forwarded-For")
		}
		if IPAddress == "" {
			IPAddress = r.RemoteAddr
		}
		IPAddress = "127.1.1.8"

		IPAddress = GetKey(IPAddress)
		fmt.Println("IP:", IPAddress)
		val, err := redis.Int(conn.Do("GET", IPAddress))

		if err != nil {
			fmt.Println("Error is", err)
			conn.Do("SET", IPAddress, 1)
			conn.Do("EXPIRE", IPAddress, EXPIRY)
		} else {
			fmt.Println(val, THRESHOLD)
			if val > THRESHOLD {
				err := errors.New("Max Rate Limiting Reached, Please try after some time")
				w.Write([]byte(err.Error()))
				return
			}
			conn.Do("SET", IPAddress, val+1)
		}
		fmt.Println("IP count:", val)
		next.ServeHTTP(w, r)
	})
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
	gmux.Path("/signin").HandlerFunc(signInUser).Methods("POST")

	for _, route := range gatewayConfig.Routes {
		backendURL, err := url.Parse(route.Target)
		if err != nil {
			log.Fatal("Error parsing backend URL:", err)
		}

		gmux.PathPrefix(route.Context).Handler(gatewayHandler(backendURL))
	}

	log.Printf("Starting smart reverse proxy on [%s]", gatewayConfig.ListenAddr)
	if err := http.ListenAndServe(gatewayConfig.ListenAddr, limitMiddleware(gmux)); err != nil {
		log.Fatalf("Unable to start server: %s", err.Error())
	}
	return
}
