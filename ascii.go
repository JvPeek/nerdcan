package main

import (
	"os"
	"log"
)

var NerdCANASCII string

func init() {
	content, err := os.ReadFile("ascii.txt")
	if err != nil {
		log.Fatalf("Error reading ascii.txt: %v", err)
	}
	NerdCANASCII = string(content)
}
