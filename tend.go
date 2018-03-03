package main

import (
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

func main() {
	dir := *flag.String("w", "", "Directory you want to watch")
	verbose := flag.Bool("v", false, "Verbose output")
	parallel := flag.Bool("p", false, "Run commands in parallel")
	countP := flag.Int("c", -1, "Number of times to watch dir")
	flag.Parse()
	count := *countP
	cmdsStr := flag.Args()
	vlog := verboseLogger(*verbose)

	if dir == "" && len(cmdsStr) > 1 {
		dir = cmdsStr[0]
		cmdsStr = cmdsStr[1:]
	}

	if dir == "" || len(cmdsStr) == 0 {
		flag.Usage()
		fmt.Println("Examples:")
		fmt.Println("  tend -w src/ \"npm run build\"")
		fmt.Println("  tend -v -w src/ make")
		fmt.Println("  tend -w src/ \"rm -rf lib\" \"npm run build:dev\"")
		return
	}

	vlog("When %s changes, I will run", dir)
	for _, c := range cmdsStr {
		vlog("    %s", c)
	}
	if *parallel {
		vlog("in parallel")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			cmds := prepareCommands(cmdsStr)
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					vlog("Change detected")
					if *parallel {
						runCommandsParallel(cmds)
					} else {
						runCommands(cmds)
					}
					count--
					if count == 0 {
						vlog("tend has run %d times. Exiting.", *countP)
						done <- true
					}
				}
			case err := <-watcher.Errors:
				log.Printf("Watching %s error: %v\n", dir, err)
			}
		}
	}()

	err = watcher.Add(dir)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

func verboseLogger(v bool) func(string, ...interface{}) {
	return func(format string, a ...interface{}) {
		if v {
			fmt.Printf(format+"\n", a...)
		}
	}
}

func prepareCommands(cmdsStr []string) []*exec.Cmd {
	cmds := make([]*exec.Cmd, len(cmdsStr))
	for i, cs := range cmdsStr {
		c := strings.Split(cs, " ")
		cmds[i] = exec.Command(c[0], c[1:]...)
	}
	return cmds
}

func runCommands(cs []*exec.Cmd) {
	for _, cmd := range cs {
		runCommand(cmd)
	}
}

func runCommandsParallel(cs []*exec.Cmd) {
	var wg sync.WaitGroup
	wg.Add(len(cs))
	for _, cmd := range cs {
		go func(cmd *exec.Cmd) {
			defer wg.Done()
			runCommand(cmd)
		}(cmd)
	}
	wg.Wait()
}

func runCommand(cmd *exec.Cmd) {
	fmt.Println(strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.Path, "encountered an error:")
		fmt.Println(err)
	}
	if len(output) != 0 {
		fmt.Print(string(output))
	}
}
