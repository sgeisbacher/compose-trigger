package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
)

const COMPOSE_FILE_NAME = "docker-compose.yml"

var REGEX_UPDATE = regexp.MustCompile("^/update/([a-zA-Z0-9_\\-]+)/?$")

var projectsBaseDir string
var port int

func main() {
	flag.StringVar(&projectsBaseDir, "projectBaseDir", "/root/", "directory where all your projects are located")
	flag.IntVar(&port, "port", 8080, "listening port")
	flag.Parse()

	http.HandleFunc("/update/", updateHandler)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r.URL.Path)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	exists, composeFilePath := projectExists(projectID)
	if !exists {
		fmt.Printf("'%v' not found", composeFilePath)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	fmt.Printf("%v: pulling ...\n", projectID)
	cmd1 := exec.Command("docker-compose", "-f", composeFilePath, "pull")
	cmd1.Start()
	cmd1.Wait()

	fmt.Printf("%v: starting ...\n", projectID)
	cmd2 := exec.Command("docker-compose", "-f", composeFilePath, "up", "-d")
	cmd2.Start()
	cmd2.Wait()
	fmt.Printf("%v: done\n", projectID)

	w.WriteHeader(http.StatusOK)
}

func parseProjectID(path string) (string, error) {
	match := REGEX_UPDATE.FindStringSubmatch(path)
	if match == nil {
		return "", fmt.Errorf("request-path '%v' does not match regex", path)
	}
	return match[1], nil
}

func projectExists(projectID string) (bool, string) {
	composeFilePath := path.Join(projectsBaseDir, projectID, COMPOSE_FILE_NAME)
	_, err := os.Stat(composeFilePath)
	return err == nil, composeFilePath
}
