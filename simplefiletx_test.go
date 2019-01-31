package simplefiletx

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestIt(t *testing.T) {
	tx := &http.Transport{}
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	tx.RegisterProtocol("file", NewSimpleFileTransport(dir))
	c := &http.Client{Transport: tx}

	patterns := []string{
		"file:" + dir + "/test.txt",
		"file:test.txt",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			resp, err := c.Get(pattern)
			if assert.NoError(t, err) {
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Body)
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				if assert.NoError(t, err) {
					assert.Equal(t, []byte("hello world\n"), body)
				}
			}
		})
	}
}

type customFileInfo struct {
	size int64
}

func (*customFileInfo) Name() string {
	return "name"
}

func (cfi *customFileInfo) Size() int64 {
	return cfi.size
}

func (*customFileInfo) Mode() os.FileMode {
	return 0
}

func (*customFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (*customFileInfo) IsDir() bool {
	return false
}

func (*customFileInfo) Sys() interface{} {
	return nil
}

type customReader struct {
	r      *bytes.Reader
	values map[string][]string
}

func (cr *customReader) Read(b []byte) (int, error) {
	return cr.r.Read(b)
}

func (cr *customReader) Close() error {
	return nil
}

func (cr *customReader) Stat() (os.FileInfo, error) {
	return &customFileInfo{cr.r.Size()}, nil
}

func (cr *customReader) GetHTTPMetadataKeys() ([]string, error) {
	retval := make([]string, 0, len(cr.values))
	for k, _ := range cr.values {
		retval = append(retval, k)
	}
	return retval, nil
}

func (cr *customReader) GetHTTPMetadata(k string) ([]string, error) {
	v, ok := cr.values[k]
	if !ok {
		return nil, errors.New("no such key")
	}
	return v, nil
}

type customOpener struct {
	values map[string][]string
}

func (co *customOpener) Open(name string) (ReaderWithStat, error) {
	return &customReader{
		bytes.NewReader([]byte("hello world\n")),
		co.values,
	}, nil
}

func TestCustomOpenerBasic(t *testing.T) {
	tx := &http.Transport{}
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)

	tx.RegisterProtocol(
		"file",
		&SimpleFileTransport{
			dir,
			&customOpener{
				map[string][]string{
					"Content-Type": []string{"text/x-test"},
				},
			},
		},
	)
	c := &http.Client{Transport: tx}

	resp, err := c.Get("file:///test.txt")
	if assert.NoError(t, err) {
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Body)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if assert.NoError(t, err) {
			assert.Equal(t, []byte("hello world\n"), body)
		}
	}
}

func TestCustomOpenerWithContentLength(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)

	patterns := []struct {
		input []string
		rv    []byte
		err   bool
	}{
		{
			nil,
			nil, true,
		},
		{
			[]string{},
			nil, true,
		},
		{
			[]string{"5"},
			[]byte("hello"), false,
		},
	}

	for i, pattern := range patterns {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			tx := &http.Transport{}
			tx.RegisterProtocol(
				"file",
				&SimpleFileTransport{
					dir,
					&customOpener{
						map[string][]string{
							"Content-Type":   []string{"text/x-test"},
							"Content-Length": pattern.input,
						},
					},
				},
			)
			c := &http.Client{Transport: tx}

			resp, err := c.Get("file:///test.txt")
			if pattern.err {
				if !assert.Error(t, err) {
					t.Fail()
				}
			} else {
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Body)
				defer resp.Body.Close()
				body := make([]byte, resp.ContentLength)
				n, err := resp.Body.Read(body)
				assert.Equal(t, resp.ContentLength, int64(n))
				if assert.NoError(t, err) {
					assert.Equal(t, pattern.rv, body)
				}
			}
		})
	}
}
