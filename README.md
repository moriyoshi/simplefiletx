## simplefiletx

[![godoc](https://godoc.org/github.com/moriyoshi/simplefiletx?status.svg)](https://godoc.org/github.com/moriyoshi/simplefiletx)

[![travis-ci](https://travis-ci.com/moriyoshi/simplefiletx.svg?branch=master)](https://travis-ci.com/moriyoshi/simplefiletx)

simplefiletx is a protocol handler implementation for Go's `net.http`.

In contrast to Go's standard `FileTransport` implementation, it does:

* Handle relative file paths like

  `file:foo/bar/relative`

  (Although the form violates RFC8089, it'd been quite common before the standard got out the door)

And it does **not**: 

* Automatically render the directory content as HTML
* Or try to find `index.html` under the directory and return its content

when the resolved path corresponds to a directory in the filesystem. 
(This is pretty much not desired behavior when it comes to simply using URLs as an universal way to describe the resource location)

### Synopsis

```golang
import "github.com/moriyoshi/simplefiletx"

tx := &http.Transport{}
tx.RegisterProtocol("file", simplefiletx.NewSimpleFileTransport("/somewhere-in-the-filesystem"))
c := &http.Client{Transport: tx}
resp, err := c.Get("file:/test.txt") // try to fetch "/test.txt"
resp, err := c.Get("file:///test.txt") // try to fetch "/test.txt"
resp, err := c.Get("file://localhost/test.txt") // try to fetch "/test.txt"
resp, err := c.Get("file:test.txt") // try to fetch "/somewhere-in-the-filesystem/test.txt"
```

### License

MIT
