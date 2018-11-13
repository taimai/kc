package handler

import (
	"net/http"
)

func Talk() string {
	return "Hello!"
}

func MsgHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(Talk()))
}

func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Server-Status", "OK")
	w.Write([]byte("OK"))
}

func StatuszHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}
