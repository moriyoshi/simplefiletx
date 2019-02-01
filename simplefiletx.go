// Copyright (c) 2019 Moriyoshi Koizumi
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.

package simplefiletx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

// ReaderWithStat requires the type provide Stat() function in addition to
// io.ReadCloser.
type ReaderWithStat interface {
	io.ReadCloser
	Stat() (os.FileInfo, error)
}

// ReaderWithSize requires the type provide Size() function in addition to
// io.ReadCloser.
type ReaderWithSize interface {
	io.ReadCloser
	Size() int64
}

// ReaderWithSize2 requires the type provide Size() function in addition to
// io.ReadCloser.
type ReaderWithSize2 interface {
	io.ReadCloser
	Size() (int64, error)
}

// A WithHTTPMetadata provides methods that return HTTP headers, which are
// eventually populated in the response.
type WithHTTPMetadata interface {
	// GetHTTPMetadataKeys() returns a set of keys allowed to give to the first argument of GetHTTPMetadata().
	// A key corresponds to a HTTP header.
	GetHTTPMetadataKeys() ([]string, error)
	// GetHTTPMetadata() returns a list of values corresponding to the header.
	GetHTTPMetadata(key string) ([]string, error)
}

// An Opener provides a single method Open() that is supposed to open a file in the filesystem.
type Opener interface {
	// Open() may return a value that also implements WithHTTPMetadata.
	Open(name string) (io.ReadCloser, error)
}

// DefaultOpener is a very basic implementation of Opener(), whose Open() method
// simply calls os.Open().
type DefaultOpener struct{}

func (*DefaultOpener) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

var defaultOpener = &DefaultOpener{}

// A SimpleFileTransport implements http.RoundTripper.
type SimpleFileTransport struct {
	// The base directory prepended to relative pathes.
	BaseDir string
	// Opener instance
	Opener Opener
}

// NewSimpleFileTransport() returns a SimpleFileTransport whose opener is a DefaultOpener.
func NewSimpleFileTransport(baseDir string) *SimpleFileTransport {
	return &SimpleFileTransport{baseDir, defaultOpener}
}

// NewResponseFromReaderWithStat() composes a http.Response from a http.Request
// and a io.Reader that also implements either ReaderWithStat, ReaderWithSize or
// ReaderWithSize2.
func NewResponseFromReaderWithStat(req *http.Request, r io.ReadCloser) (*http.Response, error) {
	header := http.Header{}
	var contentLength int64 = -1

	meta, ok := r.(WithHTTPMetadata)
	if ok {
		keys, err := meta.GetHTTPMetadataKeys()
		if err != nil {
			return nil, err
		}

		for _, k := range keys {
			values, err := meta.GetHTTPMetadata(k)
			if err != nil {
				return nil, err
			}
			if values == nil {
				return nil, fmt.Errorf("GetHTTPMetadata(%s) returned a nil slice", k)
			} else if len(values) == 0 {
				return nil, fmt.Errorf("GetHTTPMetadata(%s) returned an empty slice", k)
			}
			k = textproto.CanonicalMIMEHeaderKey(k)
			header[k] = values
			if k == "Content-Length" {
				if len(values) > 1 {
					return nil, fmt.Errorf("Content-Length cannot have multiple values")
				}
				posContentLength, err := strconv.ParseUint(values[0], 10, 63)
				if err != nil {
					return nil, fmt.Errorf("invalid value for Content-Length: %s", values[0])
				}
				contentLength = int64(posContentLength)
			}
		}
	}

	if contentLength < 0 {
		switch r := r.(type) {
		case ReaderWithStat:
			finfo, err := r.Stat()
			if err != nil {
				return nil, err
			}
			contentLength = finfo.Size()
		case ReaderWithSize:
			contentLength = r.Size()
		case ReaderWithSize2:
			var err error
			contentLength, err = r.Size()
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s: content length unknown", req.URL.String())
		}
		header["Content-Length"] = []string{strconv.FormatInt(contentLength, 10)}
	}

	return &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
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
