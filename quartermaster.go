package main

import (
	"flag"
	"fmt"
	"github.com/stevelacy/quartermaster/manager"
	"net/http"
	"os"
	"strconv"
)

func main() {
	fmt.Printf("Starting quartermaster \n")

	tokenDescription := "Authentication token for cluster" +
		"(or ENV TOKEN=<token>)"
	portDescription := "Port for manager to listen on, 9090 by default"
	memoryDescription := "Memory limit for each task, 250 by default"

	token := flag.String("token", "", tokenDescription)
	port := flag.String("port", "", portDescription)
	memory := flag.Int64("memory", 0, memoryDescription)

	if *token == "" {
		*token = os.Getenv("TOKEN")
	}

	if *port == "" {
		*port = os.Getenv("PORT")
	}

	if *memory == 0 {
		envmem := os.Getenv("MEMORY")
		if envmem == "" {
			envmem = "250"
		}
		mem, err := strconv.Atoi(envmem)
		if err != nil {
			panic(err)
		}
		*memory = int64(mem)
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
