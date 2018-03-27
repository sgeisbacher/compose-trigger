package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"

	"github.com/google/uuid"
)

const COMPOSE_FILE_NAME = "docker-compose.yml"

var REGEX_BEARER_TOKEN = regexp.MustCompile("^Bearer (.+)$")
var REGEX_UPDATE = regexp.MustCompile("^/update/([a-zA-Z0-9_\\-]+)/?$")

var projectsBaseDir string
var port int
var tokenFilePath string
var expectedToken string

func main() {
	flag.StringVar(&projectsBaseDir, "projectBaseDir", "/root/", "directory where all your projects are located")
	flag.IntVar(&port, "port", 8080, "listening port")
	flag.StringVar(&tokenFilePath, "authTokenFile", "/root/.compose-trigger.token", "file where the auth-token will be stored")
	flag.Parse()

	expectedToken = loadExpectedToken()

	http.Handle("/update/", authMiddleware(http.HandlerFunc(updateHandler)))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		match := REGEX_BEARER_TOKEN.FindStringSubmatch(authHeader)
		if match == nil {
			fmt.Printf("rejected %v: missing token\n", r.RemoteAddr)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		token := match[1]
		if token != expectedToken {
			fmt.Printf("rejected %v: invalid token\n", r.RemoteAddr)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loadExpectedToken() string {
	data, err := ioutil.ReadFile(tokenFilePath)
	if err != nil {
		fmt.Printf("could not read token-file '%v'\n", tokenFilePath)
		return generateAndWriteToken()
	}
	if _, err := uuid.ParseBytes(data); err != nil {
		fmt.Printf("invalid in token-file '%v'\n", tokenFilePath)
		return generateAndWriteToken()
	}
	token := string(data)
	fmt.Println("auth-token:", token)
	return token
}

func generateAndWriteToken() string {
	fmt.Println("generating new auth-token ...")
	token := uuid.New().String()
	ioutil.WriteFile(tokenFilePath, []byte(token), 0600)
	fmt.Println("auth-token:", token)
	return token
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
	pullCommand := exec.Command("docker-compose", "-f", composeFilePath, "pull")
	pullCommand.Start()
	pullCommand.Wait()

	fmt.Printf("%v: starting ...\n", projectID)
	upCommand := exec.Command("docker-compose", "-f", composeFilePath, "up", "-d")
	upCommand.Start()
	upCommand.Wait()
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
