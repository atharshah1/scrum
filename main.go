/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"github.com/atharshah1/scrum/cmd"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cmd.Execute()
}
