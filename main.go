package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
	"log/slog"

	"github.com/cenkalti/backoff/v4"
)


func hasFileChanged(filename string, startFileInfo *os.FileInfo) (bool, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return false, fmt.Errorf("error stating file: %w", err)
	}

	if *startFileInfo == nil {
		*startFileInfo = fileInfo
		return false, nil
	}

	if (*startFileInfo).ModTime() != fileInfo.ModTime() || (*startFileInfo).Size() != fileInfo.Size() {
		*startFileInfo = fileInfo
		return true, nil
	}

	return false, nil
}

func StartProcess(ctx context.Context, bin string, args []string) error {
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
				slog.Error("[autorestart] process exited by itself %s", e.Error())
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func runBackoff(ctx context.Context,  fn func() error) error {
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxInterval = 10 * time.Second
	backoffConfig.MaxElapsedTime = 1 * time.Minute
	b := backoff.WithContext(backoffConfig, ctx)
	return backoff.Retry(fn, b)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	// make channel which will trigger restart 
	restartChan := make(chan bool)
	// maxErrors := 10
	currentTick := time.Second * 1
	ticker := time.NewTicker(currentTick)
	bin := os.Args[1:]
	
	go func() {
		// read from channel and restart
		for range restartChan {
			slog.Info("starting process", "component", "autorestart")
			cancel()
			ctx, cancel = context.WithCancel(context.Background())
			go func ()  {
				startError := runBackoff(ctx, func() error {
					return StartProcess(ctx, bin[0], bin[1:])
				})
				if startError != nil {
					slog.Error("start error", "error", startError)
					os.Exit(1)
				}
			}()
		
		}
		slog.Info("exiting", "component", "autorestart")
	}()
	restartChan <- true

	var startFileInfo os.FileInfo
	for range ticker.C {
		changed, err := hasFileChanged(bin[0], &startFileInfo)
		if changed {
		slog.Info("checking for changes", "component", "autorestart", "changed", changed)
			restartChan <- true
		}  
		if err != nil {
			slog.Error(fmt.Sprintf("failed stat file: %s", err), "component", "autorestart")
		}
	}
}
