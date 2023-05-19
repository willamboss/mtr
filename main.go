package main

import (
	"fmt"
	"mtr/cli"
)

func main() {
	err := cli.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
	}
}
