package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	startFileInfo os.FileInfo
)

func isChangedByStat(filename string) (bool, error) {
	fileinfo, err := os.Stat(filename)
	if err == nil {
		// first update
		if startFileInfo == nil {
			startFileInfo = fileinfo
			return false, nil
		}

		if startFileInfo.ModTime() != fileinfo.ModTime() ||
			startFileInfo.Size() != fileinfo.Size() {
			startFileInfo = fileinfo
			return true, nil
		}

		return false, nil
	}

	return false, fmt.Errorf("cannot find %s: %s", filename, err)
}

func RestartByExec(ctx context.Context, bin string, args []string) error {
	binary, err := exec.LookPath(bin)
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	startErr := cmd.Start()
	if startErr != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if e.Exited() {
				fmt.Printf("[autorestart] process exited by itself %s", e.Error())
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	maxErrors := 10
	g, gctx := errgroup.WithContext(ctx)
	currentTick := time.Second * 1
	ticker := time.NewTicker(currentTick)
	bin := os.Args[1:]
	g.Go(func() error {
		return RestartByExec(gctx, bin[0], bin[1:])
	})
	errCount := 0
	for range ticker.C {
		changed, err := isChangedByStat(bin[0])
		if err != nil && changed {
			fmt.Println("[autorestart v2] restarting...")
			cancel()
			ctx, cancel = context.WithCancel(context.Background())
			if err := g.Wait(); err != nil {
				errCount += 1
				fmt.Println("[autorestart] error: ", err)
				if errCount == 5 {
					fmt.Println("[autorestart] too many errors, exiting")
					os.Exit(1)
				}
			} else {
				errCount = 0
			}
			g, gctx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return RestartByExec(gctx, bin[0], bin[1:])
			})
		} else if err != nil {
			fmt.Println("[autorestart] error: ", err)
			errCount += 1
			if errCount == maxErrors {
				fmt.Println("[autorestart] too many errors, exiting")
				os.Exit(1)
			}
		}
	}
}
