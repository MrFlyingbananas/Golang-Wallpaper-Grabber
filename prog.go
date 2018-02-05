package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const redditURL = "https://www.reddit.com/r/Animewallpaper.json?sort=hot&raw_json=1"
const basePath = "walls"

var wg sync.WaitGroup
var win *syscall.LazyProc
var workingPath string

func main() {
	workingPath, _ = os.Getwd()
	win = syscall.NewLazyDLL("user32.dll").NewProc("SystemParametersInfoW")
	rand.Seed(time.Now().UTC().UnixNano())
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, os.ModeDir)
	}
	fmt.Print("\nWelcome to the reddit anime wallpaper getter! Get new pictures? (y/n): ")
	reader := bufio.NewReader(os.Stdin)
	s, _ := reader.ReadString('\n')
	ch := s[0]
	done := make(chan byte)
	quit := make(chan byte)
	if ch == 'n' || ch == 'N' {
		go func(start chan byte) {
			start <- 0
		}(done)
	} else {
		fmt.Print("Number of pages to scan: ")
		s, _ := reader.ReadString('\n')
		i, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			fmt.Printf("%T, %v", s, s)
		}
		go getPictures(i, done)
	}
	go func(start chan byte) {
		<-start
		go func() {
			for {
				setDesktop(randFile())
				time.Sleep(time.Second * 3)
			}
		}()
		go getInput(reader, quit)
	}(done)

	<-quit
}
func getInput(r *bufio.Reader, qu chan byte) {
	for true {
		done := make(chan byte)
		fmt.Print("\nGet new pictures? (Press q to quit): ")
		s, _ := r.ReadString('\n')
		ch := s[0]
		if ch == 'q' || ch == 'Q' {
			qu <- 0
		} else if ch == 'y' || ch == 'Y' {
			fmt.Print("Number of pages to scan: ")
			s, _ := r.ReadString('\n')
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				fmt.Printf("%T, %v", s, s)
			}
			go getPictures(i, done)
			<-done
		}
	}
}
func getPictures(pages int, done chan byte) {
	makeBase()
	after := ""
	count := 0
	for pages > 0 {
		res, err := talkToReddit(after)
		if err != nil {
			log.Fatal(err, "REDDIT")
		}
		body, _ := ioutil.ReadAll(res.Body)
		str := string(body)
		add := 0
		for true {
			var imgurl string
			desktopIndex := strings.Index(str[add:], "\"link_flair_text\": \"Desktop\"")
			if desktopIndex == -1 {
				break
			} else {
				add += desktopIndex
			}
			fromRe := regexp.MustCompile("https://i\\.imgur\\.com/[A-z0-9]+(\\.jpg|\\.png)|https://i\\.redd\\.it/[A-z0-9]+(\\.jpg|.png)")
			from := fromRe.FindStringIndex(str[add:])

			if from[0] == -1 {
				break
			}
			imgurl = str[from[0]+add : from[1]+add]
			go func() {
				defer wg.Done()
				if convertLinkToImg(imgurl) != nil {
					count++
				}
			}()
			wg.Add(1)
			add += from[1]
		}

		pages--
		if pages > 0 {
			afterIndex := strings.Index(str, "after") + 5 + 4
			after = str[afterIndex : strings.Index(str[afterIndex:], "\"")+afterIndex]
		}
	}

	wg.Wait()
	fmt.Println("Added", count, "files!")
	time.Sleep(time.Second)
	done <- 0
}
func convertLinkToImg(imgurl string) error {
	var p string
	from := regexp.MustCompile("(\\.com/)|(\\.it/)").FindStringIndex(imgurl)
	p = path.Join(basePath, imgurl[from[1]+1:])
	if _, ferr := os.Stat(p); os.IsNotExist(ferr) {
		res, err := http.Get(imgurl)
		if err != nil {
			log.Fatal(err, "IMGUR")
		}
		defer res.Body.Close()
		file, err := os.Create(p)
		defer file.Close()
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(file, res.Body)
		res.Body.Close()
		println("Adding:", imgurl)
		return errors.New("file already exists")
	}
	return nil
}
func makeBase() {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		os.Mkdir(basePath, os.ModeDir)
	}
}
func talkToReddit(after string) (*http.Response, error) {
	client := http.Client{}
	link := redditURL
	if after != "" {
		link += "&" + "after=" + after
	}
	req, _ := http.NewRequest("GET", link, nil)
	req.Header.Set("User-Agent", "Wallpaper-bot-0.1")
	return client.Do(req)

}
func setDesktop(file string) {
	file = path.Join(workingPath, basePath, file)
	fileptr, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		println(err)
	}
	go func() {
		win.Call(
			uintptr(0x0014),
			uintptr(0x0000),
			uintptr(unsafe.Pointer(fileptr)),
			uintptr(0x01|0x02),
		)
	}()

}
func randFile() string {
	files, _ := ioutil.ReadDir(basePath)
	if len(files) == 0 {
		fmt.Println("\nNo files in walls, please get more :(")
		os.Exit(0)
	}
	ran := rand.Intn(len(files))
	println(files[ran].Name())
	return files[ran].Name()
}
