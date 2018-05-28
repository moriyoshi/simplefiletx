package simplefiletx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

type ReaderWithStat interface {
	io.ReadCloser
	Stat() (os.FileInfo, error)
}

type Opener interface {
	Open(name string) (ReaderWithStat, error)
}

type DefaultOpener struct{}

func (*DefaultOpener) Open(name string) (ReaderWithStat, error) {
	return os.Open(name)
}

var defaultOpener = &DefaultOpener{}

type SimpleFileTransport struct {
	BaseDir string
	Opener  Opener
}

func NewSimpleFileTransport(baseDir string) *SimpleFileTransport {
	return &SimpleFileTransport{baseDir, defaultOpener}
}

func NewResponseFromReaderWithStat(req *http.Request, r ReaderWithStat) (*http.Response, error) {
	finfo, err := r.Stat()
	if err != nil {
		return nil, err
	}
	contentLength := finfo.Size()

	return &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Length": []string{strconv.FormatInt(contentLength, 10)}},
		Body:          r,
		ContentLength: contentLength,
		Close:         true,
		Uncompressed:  true,
		Request:       req,
	}, nil
}

func (tx *SimpleFileTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != "" && req.Method != "GET" {
		return nil, fmt.Errorf("only GET method is allowed")
	}

	var pathInUrl string
	if req.URL.Path != "" {
		// file:/foo/bar
		// file:///foo/bar
		// file://localhost/foo/bar
		pathInUrl = req.URL.Path
	} else {
		// non-RFC8089 compliant but common
		// file:relative/path
		var err error
		pathInUrl, err = url.QueryUnescape(req.URL.Opaque)
		if err != nil {
			return nil, err
		}
	}

	rawComps := bytes.Split([]byte(pathInUrl), []byte{'/'})
	pathBytes := make([]byte, 0, len(pathInUrl)+len(tx.BaseDir))

	for i, comp := range rawComps {
		if i == 0 {
			if len(comp) != 0 {
				pathBytes = append(pathBytes, []byte(tx.BaseDir)...)
				pathBytes = append(pathBytes, filepath.Separator)
			}
		} else {
			if len(comp) == 0 {
				// strip consecutive slashes
				continue
			}
		}
		if i > 0 {
			pathBytes = append(pathBytes, filepath.Separator)
		}
		pathBytes = append(pathBytes, comp...)
	}

	r, err := tx.Opener.Open(string(pathBytes))
	if err != nil {
		return nil, err
	}

	return NewResponseFromReaderWithStat(req, r)
}
