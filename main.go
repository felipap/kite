package main

import (
	"fmt"
	"net/http"
	"strconv"
	"os"
)

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]
	if len(path) == 0 {
		path = "everything"
	}
	fmt.Fprintf(w, "Hi there, I love %s!", path)
}

func main() {
	port := 8080
	fmt.Printf("Server listening on port %s\n", strconv.Itoa(port))
	http.HandleFunc("/", handler)
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}
