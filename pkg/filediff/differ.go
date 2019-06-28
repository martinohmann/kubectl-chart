package filediff

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinohmann/go-difflib/difflib"
)

type Options struct {
	Context int
	Color   bool
}

type Differ struct {
	from, to string
	options  Options
}

var DefaultOptions = Options{Context: 10, Color: true}

func NewDiffer(from, to string) *Differ {
	return NewDifferWithOptions(from, to, DefaultOptions)
}

func NewDifferWithOptions(from, to string, options Options) *Differ {
	return &Differ{
		from:    from,
		to:      to,
		options: options,
	}
}

func (d *Differ) WriteTo(w io.Writer) (int64, error) {
	fromInfo, err := os.Stat(d.from)
	if err != nil {
		return 0, err
	}

	toInfo, err := os.Stat(d.to)
	if err != nil {
		return 0, err
	}

	if !fromInfo.IsDir() && !toInfo.IsDir() {
		p := pair{
			A: &fileInfo{
				AbsPath: d.from,
				Base:    filepath.Base(d.from),
			},
			B: &fileInfo{
				AbsPath: d.to,
				Base:    filepath.Base(d.to),
			},
		}

		return p.WriteTo(w)
	}

	fromFiles, err := getFileInfos(d.from, fromInfo)
	if err != nil {
		return 0, err
	}

	toFiles, err := getFileInfos(d.to, toInfo)
	if err != nil {
		return 0, err
	}

	var n int64

	for _, p := range pairs(fromFiles, toFiles) {
		nn, err := p.WriteTo(w)

		n += nn

		if err != nil {
			return n, err
		}
	}

	return n, nil
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
			return err
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
		return nil, err
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

func (p pair) WriteTo(w io.Writer) (int64, error) {
	out, err := p.diff()
	if err != nil {
		return 0, err
	}

	n, err := w.Write([]byte(out))

	return int64(n), err
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

func trimDirPrefix(dir, path string) string {
	return strings.TrimPrefix(path, filepath.Clean(dir)+"/")
}
