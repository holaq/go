package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"
)

func main() {
}

type M map[string]interface{}

func asserts(e ...interface{}) {
	if e != nil {
		for _, ee := range e {
			if ee != nil {
				if _, is := ee.(error); is {
					panic(ee)
				}
			}
		}
	}
}

func Error(msg string, args ...interface{}) error {
	if args != nil && len(args) != 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	return errors.New(msg)
}
func assertequal(obj1, obj2 interface{}, msg ...interface{}) {
	if obj1 != obj2 {
		panic(fmt.Sprintf("v1=[%v] != v2[%v]: %v", obj1, obj2, msg))
	}
}

func assert(e interface{}) {
	if e != nil {
		panic(e)
	}
}

func init() {
	rand.Seed(time.Now().Unix())
}

func RandBytes(bs []byte) {
	for i, _ := range bs {
		bs[i] = byte(rand.Int())
	}
}

func RandHex(l int) string {
	bs := make([]byte, l)
	RandBytes(bs)
	return hex.EncodeToString(bs)
}

func httpRequest(url, method string, headers map[string]string, body []byte) (resBody []byte, code int, e error) {
	var httpReq *http.Request
	var httpRes *http.Response
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	if httpReq, e = http.NewRequest(method, url, reqBody); e != nil {
	} else {
		if headers != nil {
			for k, v := range headers {
				httpReq.Header.Set(k, v)
			}
		}
		if httpRes, e = new(http.Client).Do(httpReq); e != nil {
		} else {
			code = httpRes.StatusCode
			defer httpRes.Body.Close()
			resBody, e = ioutil.ReadAll(httpRes.Body)
		}
	}
	return
}
