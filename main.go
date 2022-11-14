package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"time"
)

func main() {
	app := cli.NewApp()
	app.Name = "MCS Upgrade Tool"
	app.Usage = "Upgrade MCS contract"
	app.Compiled = time.Now()

	app.Commands = []*cli.Command{
		generateCMD(),
	}

	fmt.Println("os.Args", os.Args)

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}
}
