package simplefiletx

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIt(t *testing.T) {
	tx := &http.Transport{}
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	tx.RegisterProtocol("file", NewSimpleFileTransport(dir))
	c := &http.Client{Transport: tx}
	{
		resp, err := c.Get("file:" + dir + "/test.txt")
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
	{
		resp, err := c.Get("file:test.txt")
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
}
