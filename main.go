package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sync/errgroup"
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

func RestartByExec(ctx context.Context, bin string, args []string) error {
	binary, err := exec.LookPath(bin)
	if err != nil {
		log.Printf("[autorestart] Error: %s", err)
		return err
	}
	time.Sleep(1 * time.Second)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	startErr := cmd.Start()
	if startErr != nil {
		log.Printf("[autorestart] error: %s %v", binary, startErr)
		return err
	}

	if err := cmd.Wait(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if e.Exited() {
				fmt.Printf("[autorestart] process exited by itself %s", e.Error())
				return err
			}
		} else {
			fmt.Println("[autorestart] ", err)
			return err
		}
	}
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	g, gctx := errgroup.WithContext(ctx)

	ticker := time.NewTicker(time.Second * 1)
	bin := os.Args[1:]
	g.Go(func() error {
		return RestartByExec(gctx, bin[0], bin[1:])
	})

	for range ticker.C {
		if isChangedByStat(bin[0]) {
			fmt.Println("[autorestart] restarting...")
			cancel()
			ctx, cancel = context.WithCancel(context.Background())
			if err := g.Wait(); err != nil {
				fmt.Println("[autorestart] error: ", err)
			}
			g, gctx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return RestartByExec(gctx, bin[0], bin[1:])
			})
		}
	}
}
