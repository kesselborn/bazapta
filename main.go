package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
)

var (
	laddr        = flag.String("l", "0.0.0.0:8080", "Listen address")
	repreproPath = flag.String("p", "/srv/reprepro/internal/", "Reprepro path")
)

var id int

type indexedRequest struct {
	*http.Request
	id int
}

func main() {
	var err error

	flag.Parse()
	http.HandleFunc("/", handleRequest)

	defer func() {
		if err != nil {
			log.Fatal("GLOBAL: " + err.Error())
			os.Exit(1)
		}
	}()


	if err != nil {
		return
	}
	log.Printf("GLOBAL: listening on %s\n", *laddr)
	err = http.ListenAndServe(*laddr, nil)

}

func handleRequest(res http.ResponseWriter, req *http.Request) {
	//var distribution string
	var err error

	id += 1
	iReq := indexedRequest{req, id}

	log.Printf("GLOBAL: received request, assigning id REQ[%04d]", id)

	rePattern := regexp.MustCompile("/distributions/([^/]+)$")
	distribution := rePattern.FindStringSubmatch(iReq.URL.Path)

	switch {
	case len(distribution) > 0:
		log.Printf("REQ[%04d] detected distribution: %s", iReq.id, distribution[1])
		err = registerNewPackage(res, iReq, distribution[1])
	default:
		log.Printf("REQ[%04d] unspecified location: %s", iReq.id, distribution)
		res.Header().Set("Location", "/")
		res.WriteHeader(301)
		return
	}

	if err != nil {
		res.WriteHeader(500)
		fmt.Fprintf(res, "%s\n", err.Error())
	}
}

func registerNewPackage(res http.ResponseWriter, req indexedRequest, distribution string) (err error) {
	switch req.Method {
	case "POST":
		log.Printf("REQ[%04d] get a new package for '%s'", req.id, distribution)
		filename, err := persistFile(req)
		print(filename)
		if err != nil {
			return err
		}

	case "GET":
		log.Printf("REQ[%04d] get a new package for '%s'", req.id, distribution)
		err = listPackages(res, req, distribution)

	default:
		log.Printf("REQ[%04d] forbidden method: %s", req.id, req.Method)

		res.Header().Set("Allow", "POST,GET")
		res.WriteHeader(http.StatusMethodNotAllowed)
	}

	return
}

func listPackages(res http.ResponseWriter, req indexedRequest, distribution string) (err error) {
	path, err := exec.LookPath("reprepro")
	if err != nil {
		return
	}

	cmd := exec.Cmd{
		Path: "/usr/bin/sudo",
		Dir:  *repreproPath,
		Args: []string{path, "list", distribution},
	}
	log.Printf("REQ[%04d] executing: %#v", req.id, cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("REQ[%04d] executing: %#v", req.id, cmd)
		return
	}

	fmt.Fprintf(res, string(output))

	return
}

func persistFile(req indexedRequest) (filename string, err error) {
	src, header, err := req.FormFile("file")
	if err != nil {
		return
	}
	defer src.Close()

	filename = "/tmp/" + header.Filename
	log.Printf("REQ[%04d] saving received file to %s.", req.id, filename)
	dst, err := os.Create(filename)
	if err != nil {
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return
	}

	return
}
