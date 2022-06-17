package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var verbose = false

type sfvFile struct {
	items map[string]string
}

type fileList struct {
	root  string
	files map[string]interface{}
}

type unrar struct {
	filename string
	wd       string
}

func (u *unrar) path() string {
	return filepath.Join(u.wd, u.filename)
}

func (f *fileList) find(filter func(item string) bool) []string {
	files := []string{}
	for file := range f.files {
		if filter(file) {
			files = append(files, file)
		}
	}

	return files
}

func (f *fileList) findName(name string) []string {
	return f.find(func(item string) bool {
		return name == item
	})
}

func (f *fileList) findWithExt(ext string) []string {
	return f.find(func(item string) bool {
		fileExt := filepath.Ext(item)
		return ext == fileExt
	})

}

func (f *fileList) Path(file string) string {
	return filepath.Join(f.root, file)
}

func newSFVFile() *sfvFile {
	return &sfvFile{
		items: make(map[string]string),
	}
}

func listFiles(root string) (*fileList, error) {
	list := fileList{
		root:  root,
		files: make(map[string]interface{}),
	}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if root == path {
			return nil
		}
		name := filepath.Base(path)
		list.files[name] = nil
		return nil
	})

	return &list, nil
}

func parseSFV(filename string) (*sfvFile, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	data, err := io.ReadAll(f)
	if err != nil && err != io.EOF {
		return nil, err
	}

	content := strings.TrimSpace(string(data))

	sfv := newSFVFile()
	for _, line := range strings.Split(content, "\n") {
		parts := strings.Split(line, " ")

		sfv.items[parts[0]] = parts[1]

	}

	return sfv, nil
}

func anyMissing(sfv *sfvFile, dir *fileList) []string {
	missing := []string{}
	for sfvFile := range sfv.items {
		if _, ok := dir.files[sfvFile]; !ok {
			missing = append(missing, sfvFile)
		}
	}

	return missing
}

func findFirst[T any](list []T) (*T, error) {
	if len(list) > 0 {
		return &list[0], nil
	}

	return nil, fmt.Errorf("no first because zero length")
}

func filenameFromRar(rarPath string) (string, error) {
	cmd := exec.Command("unrar", []string{"lb", rarPath}...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("rar command failure: %w", err)
	}

	return string(out), nil

}

func extractedFile(rar string, dir *fileList) (filename string, exists bool) {
	name, err := filenameFromRar(dir.Path(rar))
	if err != nil {
		return filename, exists
	}

	names := dir.findName(name)
	if len(names) > 0 {
		filename = dir.Path(names[0])
		exists = true
	}

	return filename, exists
}

func findUnrarable(dir *fileList) (*unrar, error) {
	result := unrar{filename: "", wd: dir.root}
	sfvFile := ""
	if files := dir.findWithExt(".sfv"); len(files) > 0 {
		sfvFile = dir.Path(files[0])
	} else {
		return &result, fmt.Errorf("no .sfv files found in %s\n", dir.root)
	}

	sfv, err := parseSFV(sfvFile)
	if err != nil {
		return &result, err
	}

	missing := anyMissing(sfv, dir)
	if len(missing) > 0 {
		if verbose {
			for _, m := range missing {
				fmt.Printf("Missing files: %s\n", m)
			}
		}
		return nil, fmt.Errorf("rar files missing")
	}

	rar, err := findFirst(dir.findWithExt(".rar"))
	if err != nil {
		return &result, fmt.Errorf("no .rar file found")
	}

	file, exists := extractedFile(*rar, dir)
	if exists {
		return &result, fmt.Errorf("%q already exists", file)
	}
	result.filename = *rar

	return &result, err
}

func doUnrarAll(targets []*unrar) ([]string, error) {
	for _, t := range targets {
		fmt.Printf("Will run: %s in %s\n", t.filename, t.wd)
	}

	return nil, nil
}

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

	unrars := make([]*unrar, 0)

	skipCount := 0
	for _, target := range allDirs {
		dir, _ := listFiles(target)
		unrar, err := findUnrarable(dir)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "skipping %s\n", target)
			}
			skipCount++
			continue
		}
		fmt.Printf("unrar candidate: %s\n", unrar.path())

		unrars = append(unrars, unrar)
	}
	fmt.Fprintf(os.Stderr, "skipped %d dirs\n", skipCount)

	doUnrarAll(unrars)

	return nil
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
