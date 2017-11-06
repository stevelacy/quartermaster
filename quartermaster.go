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
	portDescription := "Port for manager to listen on, 9090 by default"
	memoryDescription := "Memory limit for each task, 250 by default"

	token := flag.String("token", "", tokenDescription)
	port := flag.String("port", "9090", portDescription)
	memory := flag.Int64("memory", 250, memoryDescription)

	if *token == "" {
		*token = os.Getenv("TOKEN")
	}

	flag.Parse()

	if *token == "" {
		fmt.Println("Error: Missing required parameters")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Println("Listening on port:", *port, "With memory limit:", *memory)

	err := http.ListenAndServe(fmt.Sprintf(":%v", *port), manager.Init(*token, *memory))
	if err != nil {
		panic(err)
	}
}
