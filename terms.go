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

	var file *os.File
	var json []byte

	switch req.Method {
	case "GET":
		if file, err = os.Open("terms/" + term + ".json"); err != nil {
			res.WriteHeader(404)
			return nil
		}

		defer file.Close()

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
