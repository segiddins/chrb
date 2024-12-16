package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/segiddins/chrb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: chrb <version>")
		os.Exit(1)
	}
	if os.Args[1] == "--list" {
		rubies, err := chrb.ListRubies()
		if err != nil {
			fmt.Println("Error listing rubies:", err)
			os.Exit(1)
		}
		for _, ruby := range rubies {
			fmt.Println(ruby)
		}
		return
	}

	ruby, err := chrb.FindRuby(os.Args[1])
	if err != nil {
		fmt.Println("Error finding ruby:", err)
		os.Exit(1)
	}
	// fmt.Println(ruby)
	env, err := ruby.Env()
	if err != nil {
		fmt.Println("Error getting environment for ruby:", err)
		os.Exit(1)
	}
	// for _, e := range env {
	// 	fmt.Println(e)
	// }

	fmt.Println(ruby.ExecPath())
	fmt.Println(env)

	args := []string{"ruby"}
	args = append(args, os.Args[2:]...)
	err = syscall.Exec(ruby.ExecPath(), args, append(os.Environ(), env...))
	if err != nil {
		fmt.Println("Error executing ruby:", err)
		os.Exit(1)
	}
}
