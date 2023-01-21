package main

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v37/github"
)

const (
	organizationName = "SolarStrike-Software"
	repositoryName   = "rom-bot"
	tmpDir           = ".tmp"
	releaseListSize  = 10
)

func main() {
	const (
		updateCmd = "update"
		checkCmd  = "check"
	)

	flag.Parse()
	action := updateCmd
	if flag.NArg() > 0 {
		action = flag.Arg(0)
	}
	tag := flag.Arg(1)

	switch action {
	case updateCmd:
		update(tag)
		break
	case checkCmd:
		check()
		break
	default:
		fmt.Printf("Invalid command: `%s`\n", action)
		break
	}
}

func update(tag string) {
	client := github.NewClient(nil)

	var ref string
	if tag == "" || tag == "latest" {
		tag = "latest"
		ref = "refs/heads/master"
	} else {
		ref = tag
	}

	opts := github.RepositoryContentGetOptions{Ref: ref}
	url, _, err := client.Repositories.GetArchiveLink(context.Background(), organizationName, repositoryName, github.Zipball, &opts, true) // (*url.URL, *Response, error) {
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)

	response, err := http.Get(url.String())
	if err != nil {
		log.Fatal(err)
	}

	defer response.Body.Close()
	fmt.Println("status", response.Status)
	if response.StatusCode != 200 {
		return
	}

	// Create the file
	zipFilename := tag + ".zip"
	os.Mkdir(tmpDir, os.ModePerm)
	out, err := os.Create(tmpDir + "\\" + zipFilename)
	if err != nil {
		fmt.Printf("err: %s\n", err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, response.Body)

	if err != nil {
		log.Fatal(err)
	}

	files, err := Unzip(tmpDir+"\\"+zipFilename, tmpDir)
	if err != nil {
		log.Fatal(err)
	}

	path := files[0]

	for _, file := range files[1:] {
		realName := file[len(path)+1:]

		if realName == "" {
			log.Fatal("Could not parse filename", file)
		}

		fi, err := os.Stat(file)
		if fi.IsDir() {
			os.Mkdir(realName, os.ModePerm)
			continue
		}

		log.Print(realName)
		if realName == "rombot_updater.exe" {
			updateSelf("rombot_updater.exe", file)
		}

		source, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer source.Close()

		dest, err := os.Create(realName)
		if err != nil {
			log.Fatal(err)
		}
		defer dest.Close()
		_, err = io.Copy(dest, source)

		if err != nil {
			log.Fatal(err)
		}
	}

	clearCache()
	log.Print("\x1b[32mAll files updated successfully!\x1b[0m")
	time.Sleep(2 * time.Second)
}

func clearCache() {
	log.Print("\x1b[32mClearing cache\x1b[0m")
	os.Remove("cache/texts.lua")
}

func updateSelf(fname string, fromPath string) {

	tmpName := fname + ".old"
	_, err := os.Stat(tmpName)
	if err != nil {
		os.Remove(tmpName)
	}

	if _, err := os.Stat(fname); err == nil {
		err = os.Rename(fname, tmpName)
		if err != nil {
			log.Fatal(err)
		}
	}

	dest, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer dest.Close()

	source, err := os.Open(fromPath)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	_, err = io.Copy(dest, source)

	if err != nil {
		log.Fatal(err)
	}

}

func check() {
	fmt.Println("Recent releases:")

	client := github.NewClient(nil)
	opts := github.ListOptions{Page: 1, PerPage: releaseListSize}
	releases, _, err := client.Repositories.ListReleases(context.Background(), organizationName, repositoryName, &opts)

	if err != nil {
		log.Fatal(err)
	}

	for _, release := range releases {
		if *release.Draft || *release.Prerelease {
			continue
		}

		fmt.Println(*release.TagName)
	}
}

func Unzip(src string, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return filenames, err
		}

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return filenames, err
		}
	}
	return filenames, nil
}
