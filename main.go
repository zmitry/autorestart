package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

var (
	startFileInfo os.FileInfo
)

func isChangedByStat(filename string) bool {
	fileinfo, err := os.Stat(filename)
	if err == nil {
		// first update
		if startFileInfo == nil {
			startFileInfo = fileinfo
			return false
		}

		if startFileInfo.ModTime() != fileinfo.ModTime() ||
			startFileInfo.Size() != fileinfo.Size() {
			startFileInfo = fileinfo
			return true
		}

		return false
	}

	log.Printf("[autorestart] cannot find %s: %s", filename, err)
	return false
}

func RestartByExec(ctx context.Context, bin string, args []string) {
	binary, err := exec.LookPath(bin)
	if err != nil {
		log.Printf("[autorestart] Error: %s", err)
		return
	}
	time.Sleep(1 * time.Second)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	startErr := cmd.Start()
	if startErr != nil {
		log.Printf("[autorestart] error: %s %v", binary, startErr)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				if e.Exited() {
					fmt.Printf("[autorestart] process exited by itself %s", e.Error())
				}
			} else {
				fmt.Println(err)
			}
		}
	}()
}

func main() {
	ticker := time.NewTicker(time.Second * 1)
	bin := os.Args[1:]
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		RestartByExec(ctx, bin[0], bin[1:])
	}()
	for range ticker.C {
		if isChangedByStat(bin[0]) {
			fmt.Println("[autorestart] restarting...")
			cancel()
			ctx, cancel = context.WithCancel(context.Background())
			RestartByExec(ctx, bin[0], bin[1:])
		}
	}
}
