package main

import (
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Library persists the reader state on the desktop, outside the app bundle.
type Library struct{ mu sync.Mutex }

func libraryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ParalinGo", "library.json"), nil
}
func (l *Library) Load() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	path, err := libraryPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}
func (l *Library) Save(data string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !json.Valid([]byte(data)) {
		return fmt.Errorf("invalid library JSON")
	}
	path, err := libraryPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "library-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.WriteString(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
func main() {
	mygo.Bind(&Library{})
	mygo.Bind(&Browser{})
	mygo.Bind(&Learning{})
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "ParalinGo", Width: 1280, Height: 850, MinWidth: 800, MinHeight: 600, BackgroundColor: "#f6f5f0", StateKey: "main", URL: "/"})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
