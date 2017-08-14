package main

import "os"
import "flag"
import "fmt"
import "github.com/stevelacy/quartermaster/manager"

func main() {
  fmt.Printf("Starting quartermaster \n")
  tokenDescription :=  "Authentication token for cluster" +
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
  manager.Init(*token, ":9090")
}
