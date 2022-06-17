package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type sfvFile struct {
	items map[string]string
}

type fileList struct {
	root  string
	files map[string]interface{}
}

func (f *fileList) findWithExt(ext string) []string {
	files := []string{}
	for file := range f.files {
		fileExt := filepath.Ext(file)
		if fileExt == ext {
			files = append(files, file)
		}
	}

	return files

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

func whatIsMissing(sfv *sfvFile, dir *fileList) []string {
	missing := []string{}
	for sfvFile := range sfv.items {
		if _, ok := dir.files[sfvFile]; !ok {
			missing = append(missing, sfvFile)
		}
	}

	return missing
}

func run(args []string) error {
	targetDir := os.Args[1]

	fmt.Printf("checking dir: %s\n", targetDir)
	list, _ := listFiles(targetDir)

	sfvFile := ""
	fmt.Printf("finding sfv file in %s\n", targetDir)
	if files := list.findWithExt(".sfv"); len(files) > 0 {
		sfvFile = list.Path(files[0])
	} else {
		return fmt.Errorf("no .sfv files found in %s\n", targetDir)
	}

	fmt.Printf("parsing %s\n", sfvFile)
	sfv, err := parseSFV(sfvFile)
	if err != nil {
		return err
	}

	for k, v := range sfv.items {
		fmt.Printf("%s %s\n", k, v)
	}

	missing := whatIsMissing(sfv, list)

	for _, m := range missing {
		fmt.Printf("missing: %s\n", m)
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		panic("need one argument")
	}

	if err := run(os.Args); err != nil {
		fmt.Printf("unexpected error: %v", err)
		os.Exit(1)
	}

}
