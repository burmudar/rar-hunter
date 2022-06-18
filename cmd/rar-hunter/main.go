package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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

func run(args []string) error {
	targetDir := os.Args[1]
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

	return rary.DoAll(unrars, os.Stdout)
}

func main() {
	if len(os.Args) < 2 {
		panic("need one argument")
	}

	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

}
