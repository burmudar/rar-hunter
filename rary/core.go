package rary

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SFVFile struct {
	items map[string]string
}

type DirSnapshot struct {
	root  string
	files map[string]interface{}
}

type Unrar struct {
	filename string
	wd       string
}

type CriteriaResult[T any] struct {
	Value    T
	Reason   string
	StringFn func(v T) string
}

func (c *CriteriaResult[T]) String() string {
	var value string
	if c.StringFn == nil {
		value = fmt.Sprintf("%v", c.Value)
	} else {
		value = c.StringFn(c.Value)
	}
	return fmt.Sprintf("Reason: %s\n", value)
}

func (c *CriteriaResult[T]) Error() error {
	return fmt.Errorf(c.String())
}

type Criteria[T any] func(dir *DirSnapshot, sfv *SFVFile) (bool, CriteriaResult[T])

func (u *Unrar) Path() string {
	return filepath.Join(u.wd, u.filename)
}

func (f *DirSnapshot) Find(filter func(item string) bool) []string {
	files := []string{}
	for file := range f.files {
		if filter(file) {
			files = append(files, file)
		}
	}

	return files
}

func (f *DirSnapshot) FindName(name string) []string {
	return f.Find(func(item string) bool {
		return name == item
	})
}

func (f *DirSnapshot) FindExt(ext string) []string {
	return f.Find(func(item string) bool {
		fileExt := filepath.Ext(item)
		return ext == fileExt
	})

}

func (f *DirSnapshot) Path(file string) string {
	return filepath.Join(f.root, file)
}

func newSFVFile() *SFVFile {
	return &SFVFile{
		items: make(map[string]string),
	}
}

func NewDirSnapshot(root string) (*DirSnapshot, error) {
	list := DirSnapshot{
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

func parseSFV(filename string) (*SFVFile, error) {
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

func anyMissing(sfv *SFVFile, dir *DirSnapshot) []string {
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

func extractedFile(rar string, dir *DirSnapshot) (filename string, exists bool) {
	name, err := filenameFromRar(dir.Path(rar))
	if err != nil {
		return filename, exists
	}

	names := dir.FindName(name)
	if len(names) > 0 {
		filename = dir.Path(names[0])
		exists = true
	}

	return filename, exists
}

func NoMissingFiles(dir *DirSnapshot, sfv *SFVFile) (bool, CriteriaResult[[]string]) {
	var result CriteriaResult[[]string]
	missing := anyMissing(sfv, dir)
	if len(missing) > 0 {
		result.Value = missing
		result.Reason = "required files were missing"
		result.StringFn = func(v []string) string {
			return fmt.Sprintf("Missing files:\n%s\n", strings.Join(v, "\n"))
		}
	}

	return len(result.Value) > 0, result
}

func NotAlreadyUnrared(dir *DirSnapshot, sfv *SFVFile) (bool, CriteriaResult[string]) {
	var result CriteriaResult[string]
	rar, err := findFirst(dir.FindExt(".rar"))
	if err != nil {
		return false, result
	}

	file, exists := extractedFile(*rar, dir)
	result.Value = file
	if exists {
		result.Reason = "file already exists"
		result.StringFn = func(v string) string { return v }
		return false, result
	}

	return true, result

}

func FindUnrarable(dir *DirSnapshot) (*Unrar, error) {
	result := Unrar{filename: "", wd: dir.root}
	sfvFile := ""
	if files := dir.FindExt(".sfv"); len(files) > 0 {
		sfvFile = dir.Path(files[0])
	} else {
		return &result, fmt.Errorf("no .sfv files found in %s\n", dir.root)
	}

	sfv, err := parseSFV(sfvFile)
	if err != nil {
		return &result, err
	}

	if ok, criteria := NoMissingFiles(dir, sfv); !ok {
		return nil, criteria.Error()
	}

	if ok, criteria := NotAlreadyUnrared(dir, sfv); !ok {
		return nil, criteria.Error()
	}

	v, err := findFirst(dir.FindExt(".rar"))
	if err != nil {
		return nil, fmt.Errorf("faile to find .rar in %s", dir.root)
	}

	result.filename = *v

	return &result, err
}

func DoAll(targets []*Unrar) ([]string, error) {
	for _, t := range targets {
		fmt.Printf("Will run: %s in %s\n", t.filename, t.wd)
	}

	return nil, nil
}
