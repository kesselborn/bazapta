package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/soundcloud/logorithm"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

const VERSION = "0.0.0"

var (
	laddr        = flag.String("l", "0.0.0.0:8080", "Listen address")
	repreproPath = flag.String("p", "/srv/reprepro/internal/", "Reprepro path")
	verbose      = flag.Bool("d", false, "Verbose debugging output")
)

var id int
var distributions []string
var logger *logorithm.L

type indexedRequest struct {
	*http.Request
	id int
}

func main() {
	var err error
	flag.Parse()

	logger = logorithm.New(os.Stdout, *verbose, "bazapta", VERSION, "bazapta", os.Getpid())

	defer func() {
		if err != nil {
			logger.Error("GLOBAL: " + err.Error())
			os.Exit(1)
		}
	}()

	distsPath := path.Join(*repreproPath, "/dists")
	dirEntries, err := ioutil.ReadDir(distsPath)
	if err != nil {
		return
	}

	for _, fileInfo := range dirEntries {
		if name := fileInfo.Name(); fileInfo.IsDir() && name[0] != '.' {
			distributions = append(distributions, name)
			logger.Info("GLOBAL: found distribution %s", name)
		}
	}

	if len(distributions) == 0 {
		err = errors.New("could not find any distributions in " + distsPath)
		return
	}

	rrPath, err := exec.LookPath("reprepro")
	if err != nil {
		return
	}

	cmd := exec.Cmd{Path: "/usr/bin/sudo", Args: []string{"/usr/bin/sudo", "-n", rrPath}}
	_, err = cmd.CombinedOutput()
	if err != nil {
		err = errors.New("need password-less sudo rights for " + rrPath)
		return
	}

	http.HandleFunc("/", handleRequest)
	logger.Info("GLOBAL: listening on %s\n", *laddr)
	err = http.ListenAndServe(*laddr, nil)

}

func handleRequest(res http.ResponseWriter, req *http.Request) {
	var err error

	id += 1
	iReq := indexedRequest{req, id}

	logger.Debug("GLOBAL: received request, assigning id REQ[%04d]", id)

	rePattern := regexp.MustCompile("/distributions/([^/]+)$")
	distribution := rePattern.FindStringSubmatch(iReq.URL.Path)

	switch {
	case len(distribution) > 0:
		logger.Debug("REQ[%04d] detected distribution: %s", iReq.id, distribution[1])
		err = distributionRequests(res, iReq, distribution[1])

	case iReq.URL.Path == "/":
		res.Header().Set("Allow", "GET")
		fmt.Fprintf(res, `{"distributions": ["/distributions/%s"]}`, strings.Join(distributions, `", /distributions/"`))

	default:
		logger.Debug("REQ[%04d] unspecified location: %s", iReq.id, distribution)
		res.Header().Set("Location", "/")
		res.WriteHeader(301)
	}

	if err != nil {
		res.WriteHeader(500)
		fmt.Fprintf(res, "%s\n", err.Error())
	}
}

func distributionRequests(res http.ResponseWriter, req indexedRequest, distribution string) (err error) {
	res.Header().Set("Allow", "GET,POST")

	switch req.Method {
	case "POST":
		logger.Debug("REQ[%04d] receiving a new package for '%s'", req.id, distribution)
		filename, err := persistFile(req)
		print(filename)
		if err != nil {
			return err
		}

	case "GET":
		logger.Debug("REQ[%04d] list packages for '%s'", req.id, distribution)
		err = listPackages(res, req, distribution)

	default:
		logger.Debug("REQ[%04d] forbidden method: %s", req.id, req.Method)

		res.WriteHeader(http.StatusMethodNotAllowed)
	}

	return
}

func listPackages(res http.ResponseWriter, req indexedRequest, distribution string) (err error) {
	rrPath, err := exec.LookPath("reprepro")
	if err != nil {
		return
	}

	cmd := exec.Cmd{
		Path: "/usr/bin/sudo",
		Dir:  *repreproPath,
		Args: []string{rrPath, "list", distribution},
	}
	logger.Debug("REQ[%04d] executing: %#v", req.id, cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debug("REQ[%04d] executing: %#v", req.id, cmd)
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
	logger.Debug("REQ[%04d] saving received file to %s.", req.id, filename)
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
