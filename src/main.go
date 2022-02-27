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

func symlinkWorker()  {
	L:
		for {
			select {
			case entry:= <-symlinkCh:
				if(loader != nil) {
					loader.Stop()
				}
				loader := makeLoader(entry.file,  entry.destinationDir)
				loader.Start()

				err := makeSymlink(entry.file, entry.destinationDir,entry.sourcePath)
				if(loader != nil) {
					loader.Stop()
				}
				
				if err != nil {
					fmt.Println(fmt.Sprint(err))
				}
			case <-symlinkDoneCh:
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

func makeLoader(preffix, suffix string) *spinner.Spinner {
	loader := spinner.New(spinner.CharSets[43], 100*time.Millisecond) 
	loader.Prefix = preffix + " "
	loader.Suffix = " " + suffix 
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
	cmd :=  exec.Command("/bin/sh", "-c", "mkdir -p " + dir)
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

