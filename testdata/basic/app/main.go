package main

import (
 "fmt"
 "example.test/basic/model"
)

func main() { fmt.Println(model.New[string]().Read()) }
