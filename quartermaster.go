package main

import (
	"flag"
	"fmt"
	"github.com/stevelacy/quartermaster/manager"
	"net/http"
	"os"
)

func main() {
	fmt.Printf("Starting quartermaster \n")
	tokenDescription := "Authentication token for cluster" +
		"(or ENV TOKEN=<token>)"
	token := flag.String("token", "", tokenDescription)
	if *token == "" {
		*token = os.Getenv("TOKEN")
	}
	flag.Parse()

	if *token == "" {
		fmt.Println("Error: Missing required parameters")
		flag.PrintDefaults()
		os.Exit(1)
	}
	port := ":9090"
	err := http.ListenAndServe(port, manager.Init(*token))
	if err != nil {
		panic(err)
	}
	fmt.Println("Listening on port", port)
}
