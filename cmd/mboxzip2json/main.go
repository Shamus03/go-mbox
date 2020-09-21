package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Shamus03/go-mbox"
)

func main() {
	if err := mainErr(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mainErr() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("missing file name")
	}

	emails, err := mbox.ExtractFile(os.Args[1])
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(emails)
}
