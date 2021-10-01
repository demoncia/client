//Super simple file list builder.
package main

import (
	"archive/zip"
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
)

type Config struct {
	Client         string `yaml:"client,omitempty"`
	DownloadPrefix string `yaml:"downloadprefix,omitempty"`
}

type FileList struct {
	Version        string      `yaml:"version,omitempty"`
	Deletes        []FileEntry `yaml:"deletes,omitempty"`
	DownloadPrefix string      `yaml:"downloadprefix,omitempty"`
	Downloads      []FileEntry `yaml:"downloads,omitempty"`
	Unpacks        []FileEntry `yaml:"unpacks,omitempty"`
}

type FileEntry struct {
	Name string `yaml:"name,omitempty"`
	Md5  string `yaml:"md5,omitempty"`
	Date string `yaml:"date,omitempty"`
	Size int64  `yaml:"size,omitempty"`
}

var ignoreList []FileEntry

var fileList FileList
var patchFile *zip.Writer

func main() {
	start := time.Now()
	err := run()
	if err != nil {
		fmt.Println("run failed:", err.Error())
		os.Exit(1)
	}
	fmt.Printf("finished in %0.2f seconds\n", time.Since(start).Seconds())
}

func run() error {
	var err error
	var out []byte
	fmt.Println("scanning directory for files...")

	path := "filelistbuilder.yml"
	inFile, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	config := Config{}
	err = yaml.Unmarshal(inFile, &config)
	if err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}

	if len(config.Client) < 1 {
		return fmt.Errorf("client not set in filelistbuilder.yml")
	}

	if len(config.DownloadPrefix) < 1 {
		return fmt.Errorf("downloadprefix not set in filelistbuilder.yml")
	}
	fileList.DownloadPrefix = config.DownloadPrefix

	err = generateIgnores("ignore.txt")
	if err != nil {
		return fmt.Errorf("generateIgnores: %w", err)
	}

	err = filepath.Walk(".", visit)
	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	h := md5.New()
	_, err = io.WriteString(h, fmt.Sprintf("%d", time.Now().Nanosecond()))
	if err != nil {
		return fmt.Errorf("write timestamp failed")
	}
	for _, d := range fileList.Downloads {
		_, err = io.WriteString(h, d.Name)
		if err != nil {
			return fmt.Errorf("write %s failed: %w", d.Name, err)
		}
	}

	fileList.Version = fmt.Sprintf("%s%x", time.Now().Format("20060102"), h.Sum(nil))

	out, err = yaml.Marshal(&fileList)
	if err != nil {
		return fmt.Errorf("yaml marshal: %w", err)
	}
	if len(fileList.Downloads) == 0 {
		return fmt.Errorf("no files found in directory")
	}

	path = "filelist_" + config.Client + ".yml"
	err = ioutil.WriteFile(path, out, 0644)
	if err != nil {
		return fmt.Errorf("writefile %s: %w", path, err)
	}

	//Now let's make patch zip.
	err = createPatch()
	if err != nil {
		return fmt.Errorf("createPatch: %w", err)
	}

	log.Println("Wrote filelist_"+config.Client+".yml and demoncia.zip with", len(fileList.Downloads), "files inside.")
	return nil
}

func createPatch() error {
	var err error
	var f io.Writer
	var buf *os.File

	buf, err = os.Create("demoncia.zip")
	if err != nil {
		return fmt.Errorf("create path.zip: %w", err)
	}

	patchFile = zip.NewWriter(buf)

	for _, download := range fileList.Downloads {
		var in io.Reader
		//fmt.Println("Adding", download.Name)
		f, err = patchFile.Create(download.Name)
		if err != nil {
			return fmt.Errorf("create %s: %w", download.Name, err)
		}

		in, err = os.Open(download.Name)
		if err != nil {
			return fmt.Errorf("open %s: %w", download.Name, err)
		}

		_, err = io.Copy(f, in)
		if err != nil {
			return fmt.Errorf("copy %s: %w", download.Name, err)
		}
	}

	//Now let's create a README.txt
	readme := "Extract the contents of demoncia.zip to play.\r\n"
	if len(fileList.Deletes) > 0 {
		readme += "Also delete the following files:\r\n"
		for _, del := range fileList.Deletes {
			readme += del.Name + "\r\n"
		}
	}

	f, err = patchFile.Create("README.txt")
	if err != nil {
		return fmt.Errorf("create README: %w", err)
	}

	_, err = f.Write([]byte(readme))
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	err = patchFile.Close()
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

func visit(path string, f os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	ignores := []string{
		"eqemupatcher.exe",
		".gitignore",
		".DS_Store",
		"xackery",
		"filelistbuilder",
		"filelist",
		"ignore.txt",
		"memorystrategy.txt",
		"sky.txt",
		"dbg.txt",
		"texture.txt",
		"eqgame.id",
		"eqgame.til",
		"eqgame.nam",
		"build.bat",
		"debug.dmp",
		"demoncia.zip",
		".sql",
		".exp",
		".ilk",
		".lib",
		".pdb",
		"uierrors.txt",
		"eqzxc.exe",
		"eqgame.exe -",
	}
	if strings.HasPrefix(path, "_") && strings.Contains(path, "\\") {
		return nil
	}
	if strings.HasSuffix(path, ".ini") && strings.Contains(path, "UI_") {
		return nil
	}
	if strings.HasSuffix(path, "_de.ini") {
		return nil
	}
	if strings.HasPrefix(path, "Logs\\") {
		return nil
	}
	for _, ig := range ignores {
		if strings.Contains(strings.ToLower(path), ig) {
			return nil
		}
	}

	if !f.IsDir() {
		for _, entry := range ignoreList {
			if path == entry.Name { //ignored file
				return nil
			}
		}
		//found a delete entry list
		if path == "delete.txt" {
			err = generateDeletes(path)
			if err != nil {
				return fmt.Errorf("generateDeletes: %w", err)
			}

			//Don't conntinue.
			return nil
		}

		download := FileEntry{
			Size: f.Size(),
			Name: path,
			Date: f.ModTime().Format("20060102"),
		}
		var md5Val string
		md5Val, err = getMd5(path)
		if err != nil {
			return fmt.Errorf("getMd5 %s: %w", path, err)
		}
		download.Md5 = md5Val

		fileList.Downloads = append(fileList.Downloads, download)
	}
	return nil
}

func getMd5(path string) (value string, err error) {

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	h := md5.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", fmt.Errorf("copy: %w", err)
	}
	value = fmt.Sprintf("%x", h.Sum(nil))
	return
}

func generateIgnores(path string) (err error) {
	//if ignore doesn't exist, no worries.
	if _, err = os.Stat(path); os.IsNotExist(err) {
		err = nil
		return
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := scanner.Text()
		if len(data) == 0 {
			continue
		}
		if strings.Contains(data, "#") { //Strip comments
			data = data[0:strings.Index(data, "#")]
		}
		if len(strings.TrimSpace(data)) < 1 { //skip empty lines
			continue
		}

		entry := FileEntry{
			Name: data,
		}
		ignoreList = append(ignoreList, entry)
	}

	err = scanner.Err()
	if err != nil {
		return fmt.Errorf("scanner: %w", err)
	}
	return
}

func generateDeletes(path string) (err error) {
	//if delete doesn't exist, no worries.
	if _, err = os.Stat(path); os.IsNotExist(err) {
		err = nil
		return
	}

	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := scanner.Text()
		if len(data) == 0 {
			continue
		}
		if strings.Contains(data, "#") { //Strip comments
			data = data[0:strings.Index(data, "#")]
		}
		if len(strings.TrimSpace(data)) < 1 { //skip empty lines
			continue
		}

		entry := FileEntry{
			Name: data,
		}

		fileList.Deletes = append(fileList.Deletes, entry)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return
}
