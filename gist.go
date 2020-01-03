package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
)

type GistCreateResponse struct {
	URL     string `json:"url"`
	HTMLUrl string `json:"html_url"`
}

// LOL.. github..
type FileContent struct {
	Content string `json:"content"`
}

func ReadFiles(filenames []string) (map[string]FileContent, error) {
	m := make(map[string]FileContent, 0)
	type job struct {
		name, content string
	}
	contentStream := make(chan job)
	noContentStream := make(chan struct{})
	errStream := make(chan error, 1) // quit on first error
	for _, file := range filenames {
		go func(mapRef *map[string]FileContent, f string) {
			contentBytes, err := ioutil.ReadFile(f)
			content := string(contentBytes)
			if err != nil {
				errStream <- err
				return
			}
			if strings.TrimSpace(content) == "" {
				// skip adding blank files
				noContentStream <- struct{}{}
			}
			contentStream <- job{name: f, content: content}
		}(&m, file)
	}
	for i := 0; i < len(filenames); i += 1 {
		select {
		case <-noContentStream: // consume no content finishes
		case content := <-contentStream:
			m[content.name] = FileContent{Content: content.content}
		case err := <-errStream:
			return m, err
		}
	}
	return m, nil
}

func CreateGist(files []string, description string, public bool, key string) (GistCreateResponse, error) {
	response := GistCreateResponse{}
	fileMap, err := ReadFiles(files)
	if err != nil {
		return response, err
	}
	if strings.TrimSpace(description) == "" {
		description = "(no description)"
	}
	obj := struct {
		Files       map[string]FileContent `json:"files"`
		Description string                 `json:"description"`
		Public      bool                   `json:"public"`
	}{
		Files:       fileMap,
		Description: description,
		Public:      public,
	}
	j, err := json.Marshal(obj)
	if err != nil {
		return response, err
	}
	req, err := http.NewRequest("POST", "https://api.github.com/gists", bytes.NewBuffer(j))
	if err != nil {
		return response, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+string(key))
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return response, err
	}
	// err = json.NewDecoder(resp.Body).Decode(&response)
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(content, &response)
	return response, err
}

func ensure(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	usr, err := user.Current()
	ensure(err)
	key, err := ioutil.ReadFile(filepath.Join(usr.HomeDir, ".secret", "gists"))
	ensure(err)
	var description string
	var public bool
	flag.StringVar(&description, "desc", "", "Gists description")
	flag.BoolVar(&public, "public", false, "Whether or not the gists should be visible (not unlisted)")
	flag.Parse()
	files := flag.Args()
	resp, err := CreateGist(files, description, public, string(key))
	ensure(err)
	// copy url to clipboard
	ensure(clipboard.WriteAll(resp.HTMLUrl))
}
