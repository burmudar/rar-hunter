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

func (u *Unrar) Path() string {
	return filepath.Join(u.wd, u.filename)
}

func (f *DirSnapshot) Find(filter func(item string) bool) []string {
	files := []string{}
	for file := range f.files {
		file = strings.TrimSpace(file)
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
		name := strings.TrimSpace(filepath.Base(path))
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

	return strings.TrimSpace(string(out)), nil

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

	if ok, criteria := MissingFiles(dir, sfv); ok {
		return nil, criteria.Error()
	}

	if ok, criteria := AlreadyUnrared(dir, sfv); ok {
		return nil, criteria.Error()
	}

	v, err := findFirst(dir.FindExt(".rar"))
	if err != nil {
		return nil, fmt.Errorf("faile to find .rar in %s", dir.root)
	}

	// dir.FindExt is a bit inconsistent. When do we need to find the relative path and when do we not ?
	result.filename = *v

	return &result, err
}

func pipeReader(cmd *exec.Cmd) (io.Reader, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	return io.MultiReader(stdout, stderr), nil

}

func DoAll(targets []*Unrar, w io.Writer) error {
	rFn := func(src string, data []byte, err error) struct {
		src  string
		data string
		err  error
	} {
		return struct {
			src  string
			data string
			err  error
		}{
			src, "data: " + string(data), err,
		}
	}
	var resultCh chan struct {
		src  string
		data string
		err  error
	} = make(chan struct {
		src  string
		data string
		err  error
	})
	var errors []error = []error{}
	for i := 0; i < len(targets); i++ {
		target := targets[i]
		fmt.Printf("unrar %s in %s\n", target.filename, target.wd)
		go func() {
			cmd := exec.Command("unrar", []string{"e", target.filename}...)
			cmd.Dir = target.wd
			// TODO: We have to read stdout and stdpipe seperately since errors are on stderr but command exits with 0
			// TODO: We have to handle  the output and error reporting better
			out, err := pipeReader(cmd)
			if err != nil {
				resultCh <- rFn(target.filename, nil, fmt.Errorf("stdout pipe: %w", err))
				return
			}
			if err := cmd.Start(); err != nil {
				data, _ := io.ReadAll(out)
				resultCh <- rFn(target.filename, data, err)
				return
			}
			data, err := io.ReadAll(out)
			if err != nil && err != io.EOF {
				resultCh <- rFn(target.filename, nil, fmt.Errorf("out read: %w", err))
			}
			err = cmd.Wait()
			resultCh <- rFn(target.filename, data, err)
		}()
	}

	count := len(targets)
	for count > 0 {
		select {
		case r := <-resultCh:
			{
				if r.err != nil {
					errors = append(errors, fmt.Errorf("[%s] did not complete successfully:  %s", r.src, r.err))
				}
				count--
			}
		}
	}

	if len(errors) > 0 {
		content := ""
		for _, e := range errors {
			content = content + e.Error() + "\n"
		}
		return fmt.Errorf("encountered %d errors\n%s\n", len(targets), content)
	}
	return nil
}
