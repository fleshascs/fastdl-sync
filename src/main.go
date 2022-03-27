package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/viper"
)


type Configurations struct {
	DestinationPath  string
	Patterns  []string
	SourcePaths  []string
}

type symlinkEntry struct {
	file string
	destinationDir string
	sourcePath  string
}


var loader *spinner.Spinner =  nil
var symlinkCh = make(chan symlinkEntry)
var symlinkDoneCh = make(chan struct{})

func startLoader(){
	if(loader == nil) {
		loader = makeLoader()
		loader.Start()
	}
}

func stopLoader(){
	if loader != nil {
		loader.Stop()
		loader = nil
	}
}

func symlinkWorker()  {
	L:
		for {
			select {
			case entry:= <-symlinkCh:
				startLoader()
				loader.Prefix = entry.file + " "
				loader.Suffix = " " + entry.destinationDir 
				// time.Sleep(1 * time.Second)
				err := makeSymlink(entry.file, entry.destinationDir,entry.sourcePath)

				if err != nil {
					stopLoader()
					fmt.Println(fmt.Sprint(err))
				}
			case <-symlinkDoneCh:
				stopLoader()
				break L
			}
		}
}

func main() {
	configuration := loadConfig()
	go symlinkWorker()
	defer func() {
		symlinkDoneCh <- struct{}{}
		fmt.Println("Complete!")
	}()

	for _, sourcePath := range configuration.SourcePaths {
		for _, pattern := range configuration.Patterns {
			path := filepath.Join(sourcePath, pattern)
			filesAndDirs, _ := filepath.Glob(path)

			for _, fileOrDir := range filesAndDirs {
				info, err := os.Stat(fileOrDir)
				if err != nil {
					fmt.Println("Error while getting Stat:", fileOrDir, err)
					continue
				}
				if info.IsDir() {
					err := filepath.Walk(fileOrDir, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if info.IsDir() {
							return nil
						}
						symlinkCh <- symlinkEntry{path, configuration.DestinationPath, sourcePath}
						return nil
					})

					if err != nil {
						fmt.Println("Error while Walking:", fileOrDir, err)
					}
				}else {
					symlinkCh <- symlinkEntry{fileOrDir, configuration.DestinationPath, sourcePath}
				}
			}
		}
	}	
}

func makeLoader() *spinner.Spinner {
	loader := spinner.New(spinner.CharSets[43], 100*time.Millisecond) 
	loader.Color("red", "bold")
	return loader
}

func loadConfig() Configurations {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.SetConfigType("yml")
	
	var configuration Configurations

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	
	err := viper.Unmarshal(&configuration)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	return configuration
}


func makeSymlink(file, destinationDir, sourcePath  string)  error {
	relativePath := strings.Replace(file, filepath.Join(sourcePath), "", 1)
	destination := filepath.Join(destinationDir, relativePath)
	dir, _ := filepath.Split(destination)		
	cmd :=  exec.Command("/bin/sh", "-c", "mkdir -p \"" + dir + "\"")
	
	if err := cmd.Run(); err != nil {
		return errors.New("Error while creating DestinationPath: " + dir + " : " + fmt.Sprint(err))
	}
	
	if _, err := os.Lstat(destination); err == nil {
		os.Remove(destination)
	}

	if err := os.Symlink(file, destination); err != nil {
		return errors.New("Error while creating symlink: " + file + " -> "+ destination + " " +  fmt.Sprint(err))
	}
    return nil
}

