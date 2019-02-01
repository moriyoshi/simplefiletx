package simplefiletx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

type customReaderWithStat struct {
	r      *bytes.Reader
	err    error
	values map[string][]string
}

func (cr *customReaderWithStat) Read(b []byte) (int, error) {
	return cr.r.Read(b)
}

func (cr *customReaderWithStat) Close() error {
	return nil
}

func (cr *customReaderWithStat) Stat() (os.FileInfo, error) {
	return &customFileInfo{cr.r.Size()}, cr.err
}

func (cr *customReaderWithStat) GetHTTPMetadataKeys() ([]string, error) {
	retval := make([]string, 0, len(cr.values))
	for k, _ := range cr.values {
		retval = append(retval, k)
	}
	return retval, nil
}

func (cr *customReaderWithStat) GetHTTPMetadata(k string) ([]string, error) {
	v, ok := cr.values[k]
	if !ok {
		return nil, errors.New("no such key")
	}
	return v, nil
}

type customOpenerReaderWithStat struct {
	err    error
	values map[string][]string
}

func (co *customOpenerReaderWithStat) Open(name string) (io.ReadCloser, error) {
	return &customReaderWithStat{
		bytes.NewReader([]byte("hello world\n")),
		co.err,
		co.values,
	}, nil
}

type customReaderWithSize struct {
	r      *bytes.Reader
	values map[string][]string
}

func (cr *customReaderWithSize) Read(b []byte) (int, error) {
	return cr.r.Read(b)
}

func (cr *customReaderWithSize) Close() error {
	return nil
}

func (cr *customReaderWithSize) Size() int64 {
	return cr.r.Size()
}

func (cr *customReaderWithSize) GetHTTPMetadataKeys() ([]string, error) {
	retval := make([]string, 0, len(cr.values))
	for k, _ := range cr.values {
		retval = append(retval, k)
	}
	return retval, nil
}

func (cr *customReaderWithSize) GetHTTPMetadata(k string) ([]string, error) {
	v, ok := cr.values[k]
	if !ok {
		return nil, errors.New("no such key")
	}
	return v, nil
}

type customOpenerReaderWithSize struct {
	values map[string][]string
}

func (co *customOpenerReaderWithSize) Open(name string) (io.ReadCloser, error) {
	return &customReaderWithSize{
		bytes.NewReader([]byte("hello world\n")),
		co.values,
	}, nil
}

type customReaderWithSize2 struct {
	r      *bytes.Reader
	err    error
	values map[string][]string
}

func (cr *customReaderWithSize2) Read(b []byte) (int, error) {
	return cr.r.Read(b)
}

func (cr *customReaderWithSize2) Close() error {
	return nil
}

func (cr *customReaderWithSize2) Size() (int64, error) {
	return cr.r.Size(), cr.err
}

func (cr *customReaderWithSize2) GetHTTPMetadataKeys() ([]string, error) {
	retval := make([]string, 0, len(cr.values))
	for k, _ := range cr.values {
		retval = append(retval, k)
	}
	return retval, nil
}

func (cr *customReaderWithSize2) GetHTTPMetadata(k string) ([]string, error) {
	v, ok := cr.values[k]
	if !ok {
		return nil, errors.New("no such key")
	}
	return v, nil
}

type customOpenerReaderWithSize2 struct {
	err    error
	values map[string][]string
}

func (co *customOpenerReaderWithSize2) Open(name string) (io.ReadCloser, error) {
	return &customReaderWithSize2{
		bytes.NewReader([]byte("hello world\n")),
		co.err,
		co.values,
	}, nil
}

func TestCustomOpenerBasic(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)

	for i, expectedErr := range []error{nil, fmt.Errorf("error")} {
		openers := []func(map[string][]string) Opener{
			func(values map[string][]string) Opener {
				return &customOpenerReaderWithStat{expectedErr, values}
			},
			func(values map[string][]string) Opener {
				return &customOpenerReaderWithSize{values}
			},
			func(values map[string][]string) Opener {
				return &customOpenerReaderWithSize2{expectedErr, values}
			},
		}

		for j, opener := range openers {
			t.Run(fmt.Sprintf("%d-%d", i, j), func(t *testing.T) {
				tx := &http.Transport{}
				tx.RegisterProtocol(
					"file",
					&SimpleFileTransport{
						dir,
						opener(
							map[string][]string{
								"Content-Type": []string{"text/x-test"},
							},
						),
					},
				)
				c := &http.Client{Transport: tx}

				resp, err := c.Get("file:///test.txt")
				if expectedErr == nil || j == 1 {
					if assert.NoError(t, err) {
						assert.NotNil(t, resp)
						assert.NotNil(t, resp.Body)
						defer resp.Body.Close()
						body, err := ioutil.ReadAll(resp.Body)
						if assert.NoError(t, err) {
							assert.Equal(t, []byte("hello world\n"), body)
						}
					}
				} else {
					if !assert.Error(t, err) {
						t.FailNow()
					}
				}
			})
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
					&customOpenerReaderWithStat{
						nil,
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
