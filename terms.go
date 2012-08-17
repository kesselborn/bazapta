package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func termRequests(res http.ResponseWriter, req indexedRequest, term string) (err error) {
	res.Header().Set("Allow", "GET")
	res.Header().Set("Content-Type", "application/json")

	if term != "DebianPackage" {
		res.WriteHeader(404)
		return
	}

	var file *os.File
	var json []byte

	switch req.Method {
	case "GET":
		file, err = os.Open("terms/DebianPackage.json")
		defer func() {
			file.Close()
		}()
		if err != nil {
			return
		}

		reader := bufio.NewReader(file)
		if json, err = ioutil.ReadAll(reader); err != nil {
			return
		}

		fmt.Fprintf(res, string(json))
	default:
		res.WriteHeader(405)
	}

	return
}
