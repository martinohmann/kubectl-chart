package diff

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// PathDiffer recursively diffs files in paths.
type PathDiffer struct {
	From, To string
}

// NewPathDiffer creates a new PathDiffer for the two paths provided. If both paths are
// files, it will just produce a diff of them. If either of the two is a
// directory, the differ will recursively walk directories and diff all
// contained files.
func NewPathDiffer(from, to string) *PathDiffer {
	return &PathDiffer{
		From: from,
		To:   to,
	}
}

// Print implements Differ. It writes diffs for all files in the from and to
// paths to w using p.
func (d *PathDiffer) Print(p Printer, w io.Writer) error {
	fromInfo, err := os.Stat(d.From)
	if err != nil {
		return errors.Wrap(err, "while creating path diff")
	}

	toInfo, err := os.Stat(d.To)
	if err != nil {
		return errors.Wrap(err, "while creating path diff")
	}

	var pairs []pair

	if !fromInfo.IsDir() && !toInfo.IsDir() {
		p := pair{
			A: &fileInfo{
				AbsPath: d.From,
				Base:    filepath.Base(d.From),
			},
			B: &fileInfo{
				AbsPath: d.To,
				Base:    filepath.Base(d.To),
			},
		}

		pairs = []pair{p}
	} else {
		fromFiles, err := getFileInfos(d.From, fromInfo)
		if err != nil {
			return err
		}

		toFiles, err := getFileInfos(d.To, toInfo)
		if err != nil {
			return err
		}

		pairs = makePairs(fromFiles, toFiles)
	}

	for _, pair := range pairs {
		err = d.print(p, w, pair)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *PathDiffer) print(p Printer, w io.Writer, pair pair) error {
	var A, B []byte
	var err error

	s := Subject{}

	if pair.A != nil {
		A, err = ioutil.ReadFile(pair.A.AbsPath)
		if err != nil {
			return errors.Wrap(err, "while reading file for path diff")
		}

		s.A = string(A)
		s.FromFile = pair.A.Base
	}

	if pair.B != nil {
		B, err = ioutil.ReadFile(pair.B.AbsPath)
		if err != nil {
			return errors.Wrap(err, "while reading file for path diff")
		}

		s.B = string(B)
		s.ToFile = pair.B.Base
	}

	return p.Print(s, w)
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

func collectFileInfos(dir string) ([]fileInfo, error) {
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
			return errors.Wrap(err, "while collecting file info for path diff")
		}

		fileInfos = append(fileInfos, fileInfo{
			AbsPath: absPath,
			Base:    trimDirPrefix(dir, path),
		})

		return nil
	})

	return fileInfos, err
}

func getFileInfos(path string, info os.FileInfo) ([]fileInfo, error) {
	if info.IsDir() {
		return collectFileInfos(path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(err, "while collecting file info for path diff")
	}

	fi := fileInfo{
		AbsPath: absPath,
		Base:    filepath.Base(path),
	}

	return []fileInfo{fi}, nil
}

type pair struct {
	A, B *fileInfo
}

func makePairs(from, to fileInfos) []pair {
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

func trimDirPrefix(dir, path string) string {
	return strings.TrimPrefix(path, filepath.Clean(dir)+"/")
}
