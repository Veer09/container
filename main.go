package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Veer09/container/internal"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name: "Container",
		Usage: "Run container",
		Commands: []*cli.Command{
			image.PullCommand,
		},	
	}
	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		fmt.Println("Error: ", err)
	}
}
