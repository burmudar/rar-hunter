package rary

import (
	"fmt"
	"strings"
)

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
	result.StringFn = func(v string) string { return v }
	rar, err := findFirst(dir.FindExt(".rar"))
	if err != nil {
		return false, result
	}

	name, err := filenameFromRar(dir.Path(*rar))
	if err != nil {
		result.Reason = "problem getting rar filename"
		return false, result
	}
	names := dir.FindName(name)
	if len(names) > 0 {
		name = dir.Path(names[0])
		result.Reason = "file already exists"
		return false, result
	}
	result.Value = name
	return true, result

}
