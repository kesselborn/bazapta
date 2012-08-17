package main

import (
	"encoding/json"
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
	// regex for entries like: squeeze|main|i386: hadoop-0.20-jobtracker 0.20.2+923.97-1
	listRe = regexp.MustCompilePOSIX("(.*)\\|(.*)\\|(.*): (.*) (.*)$")
	// regex for package urls like: /dists/squeeze/main/3w-sas_3.26.00.028-2.6.26-3_arch.deb
	packageUrl = regexp.MustCompilePOSIX("/dists/(.*)/(.*)/(.*)_(.*)_(.*)\\.deb$")
	// regex for package urls like: /dists/squeeze/main/3w-sas_3.26.00.028-2.6.26-3_arch
	packageInfoUrl = regexp.MustCompilePOSIX("/dists/(.*)/(.*)/(.*)_(.*)_([^\\.]*)$")
	skipLineErr    = errors.New("Skip this line")
)

var id int
var dists []string
var logger *logorithm.L
var rrPath string

type dist struct {
	Name string
	Url  string
}

type ListEntry struct {
	Dist        dist
	Component   string
	Arch        string
	Name        string
	Version     string
	Url         string
	DownloadUrl string
}

func (le *ListEntry) fillUrls(req indexedRequest) {
	le.Dist.Url = "http://" + req.Host + "/dists/" + le.Dist.Name
	le.Url = "http://" + req.Host + "/dists/" + le.Dist.Name + "/" + le.Component + "/" + le.Name + "_" + le.Version + "_" + le.Arch
	le.DownloadUrl = "http://" + req.Host + "/dists/" + le.Dist.Name + "/" + le.Component + "/" + le.Name + "_" + le.Version + "_" + le.Arch + ".deb"
}

func pathToListEntry(path string) (le ListEntry, err error) {
	parsedUrl := packageUrl.FindStringSubmatch(path)
	if len(parsedUrl) != 6 {
		logger.Debug("Regex parsing only found %d sub-matches: %#v", len(parsedUrl), parsedUrl)
		err = skipLineErr
		return
	}

	le = ListEntry{
		Dist:      dist{parsedUrl[1], ""},
		Component: parsedUrl[2],
		Name:      parsedUrl[3],
		Version:   parsedUrl[4],
		Arch:      parsedUrl[3],
	}

	return
}

func parseListEntry(req indexedRequest, line string, distribution string) (le ListEntry, err error) {
	parsedEntry := listRe.FindStringSubmatch(line)
	if len(parsedEntry) != 6 {
		err = skipLineErr
		return
	}

	le = ListEntry{
		Dist:      dist{parsedEntry[1], ""},
		Component: parsedEntry[2],
		Arch:      parsedEntry[3],
		Name:      parsedEntry[4],
		Version:   parsedEntry[5],
	}

	le.fillUrls(req)

	return
}

type List []ListEntry

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

	rrPath, err = exec.LookPath("reprepro")
	if err != nil {
		return
	}

	err = checkPreConditions()
	if err != nil {
		return
	}

	http.HandleFunc("/", handleRequest)
	logger.Info("GLOBAL: listening on %s\n", *laddr)

	err = http.ListenAndServe(*laddr, nil)
}

func checkPreConditions() (err error) {

	err = checkRepreproPaths()
	if err != nil {
		return
	}

	return
}

func checkRepreproPaths() (err error) {
	distsPath := path.Join(*repreproPath, "/dists")
	dirEntries, err := ioutil.ReadDir(distsPath)
	if err != nil {
		return
	}

	for _, fileInfo := range dirEntries {
		if name := fileInfo.Name(); fileInfo.IsDir() && name[0] != '.' {
			dists = append(dists, name)
			logger.Info("GLOBAL: found distribution %s", name)
		}
	}

	if len(dists) == 0 {
		err = errors.New("could not find any dists in " + distsPath)
		return
	}

	return
}

func handleRequest(res http.ResponseWriter, req *http.Request) {
	var err error

	id += 1
	iReq := indexedRequest{req, id}

	logger.Debug("GLOBAL: received request, assigning id REQ[%04d]", id)

	rePattern := regexp.MustCompile("/dists/([^/]+)")
	distribution := rePattern.FindStringSubmatch(iReq.URL.Path)

	termPattern := regexp.MustCompile("^/terms/([^/]+)$")
	term := termPattern.FindStringSubmatch(iReq.URL.Path)

	switch {
	case len(distribution) > 0:
		foundDist := false
		for _, d := range dists {
			if d == distribution[1] {
				logger.Debug("REQ[%04d] verified %s is a supported distribution", iReq.id, distribution[1])
				foundDist = true
			}
		}
		if !foundDist {
			err = errors.New("unsupported distribution: '" + distribution[1] + "'")
			break
		}

		logger.Debug("REQ[%04d] detected distribution: %s", iReq.id, distribution[1])
		err = distributionRequests(res, iReq, distribution[1])

	case iReq.URL.Path == "/":
		res.Header().Set("Allow", "GET")
		res.Header().Set("Content-Type", "application/json")

		distTypes := make([]dist, len(dists))
		for i, distName := range dists {
			distTypes[i] = dist{distName, "http://" + iReq.Host + "/dists/" + distName}
		}

		json, err := json.MarshalIndent(distTypes, "", "  ")
		if err != nil {
			return
		}
		fmt.Fprintf(res, string(json))

	case len(term) > 0:
		err = termRequests(res, iReq, term[1])

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
		var filename string
		filename, err = persistFile(req)
		if err != nil {
			return err
		}
		err = registerPackage(req, distribution, filename)

	case "GET":
		logger.Debug("REQ[%04d] action: list packages for '%s'", req.id, distribution)
		err = listPackages(res, req, distribution)

	case "DELETE":
		logger.Debug("REQ[%04d] action: delete packages out of '%s'", req.id, distribution)
		err = deletePackage(res, req, distribution)

	default:
		logger.Debug("REQ[%04d] forbidden method: %s", req.id, req.Method)

		res.WriteHeader(http.StatusMethodNotAllowed)
	}

	return
}

func deletePackage(res http.ResponseWriter, req indexedRequest, distribution string) (err error) {
	le, err := pathToListEntry(req.URL.Path)
	if err != nil {
		logger.Error("REQ[%04d] error converting '%s' to list entry", req.id, req.URL.Path)
		return
	}

	cmd := exec.Cmd{
		Path: rrPath,
		Dir:  *repreproPath,
		Args: []string{rrPath, "remove", le.Dist.Name, "-A", le.Arch, le.Name},
	}
	logger.Debug("REQ[%04d] executing: %s", req.id, strings.Join(cmd.Args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("REQ[%04d] executing: %s caused error %s", req.id, strings.Join(cmd.Args, " "), output)
	}

	return
}

func listPackages(res http.ResponseWriter, req indexedRequest, distribution string) (err error) {
	cmd := exec.Cmd{
		Path: rrPath,
		Dir:  *repreproPath,
		Args: []string{rrPath, "list", distribution},
	}
	logger.Debug("REQ[%04d] executing: %s", req.id, strings.Join(cmd.Args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("REQ[%04d] executing: %s caused error %s", req.id, strings.Join(cmd.Args, " "))
		return
	}

	lines := strings.Split(string(output), "\n")
	list := make(List, len(lines))

	for i, line := range lines {
		le, err := parseListEntry(req, line, distribution)
		if err == skipLineErr {
			continue
		}
		if err != nil {
			return err
		}

		list[i] = le
	}

	json, err := json.MarshalIndent(list, "", " ")
	if err != nil {
		return
	}

	res.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(res, string(json))

	return
}

func registerPackage(req indexedRequest, distribution, filename string) (err error) {
	cmd := exec.Cmd{
		Path: rrPath,
		Dir:  *repreproPath,
		Args: []string{rrPath, "includedeb", distribution, filename},
	}
	logger.Debug("REQ[%04d] executing: %s", req.id, strings.Join(cmd.Args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("REQ[%04d] executing: %s caused error %s", req.id, strings.Join(cmd.Args, " "), output)
	}

	if skipped, _ := regexp.MatchString("^Skipping", string(output)); skipped {
		logger.Error("REQ[%04d] executing: %s caused error %s", req.id, strings.Join(cmd.Args, " "), output)
		err = errors.New("Error adding new package: " + string(output))
	}

	return
}

func persistFile(req indexedRequest) (filename string, err error) {
	src, header, err := req.FormFile("file")
	if err != nil {
		logger.Error("REQ[%04d] error getting file from request: %#v / %#v.", req.id, req, req.Header)
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
