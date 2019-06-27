package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinohmann/go-difflib/difflib"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s [from] [to]\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	differ := newDiffer(os.Args[1], os.Args[2])

	n, err := differ.diff()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if n > 0 {
		// Exit with status 1 to indicate that there are changes
		os.Exit(1)
	}
}

type differ struct {
	from, to string
}

func newDiffer(from, to string) *differ {
	return &differ{
		from: from,
		to:   to,
	}
}

func (d *differ) diff() (int, error) {
	var diffBytes int

	fromFiles, err := getFileInfos(d.from)
	if err != nil {
		return 0, err
	}

	toFiles, err := getFileInfos(d.to)
	if err != nil {
		return 0, err
	}

	for _, p := range pairs(fromFiles, toFiles) {
		out, err := p.diff()
		if err != nil {
			return diffBytes, err
		}

		diffBytes += len(out)

		if len(out) > 0 {
			fmt.Println(out)
		}
	}

	return diffBytes, nil
}

type fileInfo struct {
	AbsPath string
	Base    string
}

func (f fileInfo) matches(o fileInfo) bool {
	return f.Base == o.Base
}

type fileInfos []fileInfo

func (f fileInfos) findMatching(needle fileInfo) (fileInfo, bool) {
	for _, elem := range f {
		if elem.matches(needle) {
			return elem, true
		}
	}

	return fileInfo{}, false
}

func walkDir(dir string) ([]fileInfo, error) {
	fileInfos := make([]fileInfo, 0)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		fileInfos = append(fileInfos, fileInfo{
			AbsPath: absPath,
			Base:    trimPrefixDir(dir, path),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return fileInfos, nil
}

func getFileInfos(path string) ([]fileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return walkDir(path)
	}

	return []fileInfo{{AbsPath: absPath, Base: filepath.Base(path)}}, nil
}

type pair struct {
	A, B *fileInfo
}

func (p pair) diff() (string, error) {
	var A, B []byte
	var err error

	fromFile, toFile := "<created>", "<deleted>"

	if p.A != nil {
		A, err = ioutil.ReadFile(p.A.AbsPath)
		if err != nil {
			return "", err
		}

		fromFile = p.A.Base
	}

	if p.B != nil {
		B, err = ioutil.ReadFile(p.B.AbsPath)
		if err != nil {
			return "", err
		}

		toFile = p.B.Base
	}

	unifiedDiff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(A)),
		B:        difflib.SplitLines(string(B)),
		FromFile: fromFile,
		ToFile:   toFile,
		Context:  10,
		Color:    true,
	}

	return difflib.GetUnifiedDiffString(unifiedDiff)
}

func pairs(from, to fileInfos) []pair {
	pairs := make([]pair, 0)

	for _, infoA := range from {
		infoA := infoA
		info := pair{A: &infoA}
		if infoB, ok := to.findMatching(infoA); ok {
			info.B = &infoB
		}

		pairs = append(pairs, info)
	}

	for _, infoB := range to {
		infoB := infoB
		if _, ok := from.findMatching(infoB); !ok {
			pairs = append(pairs, pair{B: &infoB})
		}
	}

	return pairs
}

func trimPrefixDir(dir, path string) string {
	return strings.TrimPrefix(path, filepath.Clean(dir)+"/")
}
