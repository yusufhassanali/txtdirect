package txtdirect

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/mholt/caddy/caddyhttp/proxy"
	"gopkg.in/src-d/go-git.v4"
)

type ModProxy struct {
	Enable bool
	Path   string
	Cache  string
}

type Module struct {
	Path      string
	Version   string
	LocalPath string
}

type ModuleHandler interface {
	proxy() error
	fetch() error
	zip() error
}

func gomods(w http.ResponseWriter, r *http.Request, path string, c Config) error {
	pathSlice := strings.Split(path, "/")[1:] // [1:] ignores the empty slice item
	var moduleName string
	var fileName string
	for k, v := range pathSlice {
		if v == "@v" {
			fileName = pathSlice[k+1]
			break
		}
		moduleName = strings.Join([]string{moduleName, v}, "/")
	}
	localPath := fmt.Sprintf("%s/%s", c.ModProxy.Cache, moduleName[1:])
	m := Module{
		Path:      moduleName[1:], // [1:] ignores "/" at the beginning of url
		LocalPath: localPath,
		Version:   strings.Split(fileName, ".")[0], // Gets version number from last part of the path
	}
	err := m.proxy(w, r, fileName)
	if err != nil {
		return fmt.Errorf("unable to proxy the request: %s", err.Error())
	}

	return nil
}

func (m Module) proxy(w http.ResponseWriter, r *http.Request, fileName string) error {
	if fileName == "list" {
		u, err := url.Parse(fmt.Sprintf("https://%s/@v/%s", m.Path, fileName))
		if err != nil {
			return fmt.Errorf("unable to parse the url: %s", err.Error())
		}
		r.URL.Path = "" // FIXME: Reconsider this part
		reverseProxy := proxy.NewSingleHostReverseProxy(u, "", proxyKeepalive, proxyTimeout)
		reverseProxy.ServeHTTP(w, r, nil)
		return nil
	}
}

func (m Module) fetch() error {
	if _, err := os.Stat(m.LocalPath); !os.IsNotExist(err) {
		err := os.MkdirAll(m.LocalPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to create directory: %s", m.LocalPath)
		}
		// TODO: Support Auth for private modules
		_, err = git.PlainClone(m.LocalPath, false, &git.CloneOptions{
			URL:      fmt.Sprintf("https://%s", m.Path),
			Progress: os.Stdout,
		})
		if err != nil {
			return fmt.Errorf("unable to clone the module's repository: %s", err.Error())
		}
		return nil
	}
	// TODO: Change working branch based on the requested version
	r, err := git.PlainOpen(m.LocalPath)
	if err != nil {
		return fmt.Errorf("unable to open the module's repository: %s", err.Error())
	}
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("unable to get module's current branch: %s", err.Error())
	}
	err = w.Pull(&git.PullOptions{RemoteName: "origin"})
	if err != nil {
		return fmt.Errorf("unable to get module's latest changes: %s", err.Error())
	}
	return nil
}
