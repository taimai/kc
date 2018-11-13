package main

import (
	. "config"
	"fmt"
	"handler"
	"log"
	"net/http"
	"server"
	"time"
)

func main() {
	Config = LoadConfig("config/config.json")
	server.Setup()

	ticker := time.NewTicker(time.Millisecond * 40000)
	go func() {
		var counter, maxCounter int = 0, 1000 // 1000*40s is almost half day
		server.GetServices()
		for range ticker.C {
			server.GetServices()
			counter++
			if counter > maxCounter {
				server.ClearServices()
				counter = 0
				log.Printf("==> Data reloaded <==\n")
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.HealthzHandler)
	mux.HandleFunc("/statusz", handler.StatuszHandler)
	mux.HandleFunc("/cnames", server.ListCnamesHandler)
	mux.HandleFunc("/", handler.MsgHandler)
	log.Printf("Listening on port %d\n", Config.HttpPort)
	listenAddr := fmt.Sprintf(":%d", Config.HttpPort)
	s := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	log.Fatalln(s.ListenAndServe().Error())
}
