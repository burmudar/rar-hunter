package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/burmudar/rar-hunter/eventbus"
	rary "github.com/burmudar/rar-hunter/rary"
)

func allDirs(start string) []string {
	dirs := []string{start}
	filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if start == path {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}

		return nil
	})

	return dirs
}

func run(ctx context.Context, args []string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)

	eventbus.Default().Start(ctx)

	go func() {
		<-ctx.Done()
		cancel()
		eventbus.Default().Stop(5 * time.Second)
	}()

	targetDir := args[1]
	allDirs := allDirs(targetDir)

	unrars := make([]*rary.Unrar, 0)

	skipCount := 0
	for _, target := range allDirs {
		dir, _ := rary.NewDirSnapshot(target)
		unrar, err := rary.FindUnrarable(dir)
		if err != nil {
			//fmt.Fprintf(os.Stderr, "skipping %s\n", target)
			skipCount++
			continue
		}

		unrars = append(unrars, unrar)
	}
	fmt.Fprintf(os.Stderr, "skipped %d dirs\n", skipCount)

	// make rary context aware
	return rary.DoAll(unrars, os.Stdout)
}

func main() {
	if len(os.Args) < 2 {
		panic("need one argument")
	}

	ctx := context.Background()
	if err := run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

}
