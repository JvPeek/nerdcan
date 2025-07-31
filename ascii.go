package main

import (
	_ "embed"
)

//go:embed ascii.txt
var embeddedASCII string

var NerdCANASCII string

func init() {
	NerdCANASCII = embeddedASCII
}
