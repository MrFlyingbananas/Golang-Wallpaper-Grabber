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

var win *syscall.LazyProc
var workingPath string
var console *bufio.Reader

const changeIntervalSeconds = 10

func init() {
	var err error

	workingPath, err = os.Getwd()
	check(err)

	console = bufio.NewReader(os.Stdin)

	win = syscall.NewLazyDLL("user32.dll").NewProc("SystemParametersInfoW")

	rand.Seed(time.Now().UTC().UnixNano())

	makeBase()
}

func main() {
	fmt.Println("Welcome to the reddit anime wallpaper getter!")
	s, err := getInput("Enter background change interval in minutes: ")
	check(err)
	changeInterval, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	check(err)
	go func() {
		for {
			setDesktop(randFile())
			time.Sleep(time.Second * time.Duration(60*changeInterval))
		}
	}()
	for {
		if loop() {
			os.Exit(0)
		}
	}
}

func loop() bool {
	s, err := getInput("Get new pictures? (Press q to quit): ")
	check(err)
	ch := s[0]
	if ch == 'q' || ch == 'Q' {
		return true
	} else if ch == 'y' || ch == 'Y' {
		scanPages()
	}
	return false
}

func scanPages() {
	s, err := getInput("Number of pages to scan: ")
	check(err)
	i, err := strconv.Atoi(strings.TrimSpace(s))
	check(err)
	getPictures(i)

}

func getPictures(pages int) {
	var wg sync.WaitGroup
	after := ""
	count := 0
	wg.Add(pages)
	for i := 0; i < pages; i++ {
		res, err := talkToReddit(after)
		check(err, "REDDIT")
		body, err := ioutil.ReadAll(res.Body)
		check(err)
		str := string(body)
		numAdded := make(chan int)
		go func() {
			defer wg.Done()
			parsePage(str, numAdded)
		}()
		for i := range numAdded {
			count += i
		}
		if i+1 < pages {
			afterIndex := strings.Index(str, "after") + 5 + 4
			after = str[afterIndex : strings.Index(str[afterIndex:], "\"")+afterIndex]
		}

	}

	wg.Wait()
	fmt.Println("Added", count, "files!")
	time.Sleep(time.Second)
}

func parsePage(pageString string, imagesAdded chan<- int) {
	count := 0
	add := 0
	var wg sync.WaitGroup
	for {
		desktopIndex := strings.Index(pageString[add:], "\"link_flair_text\": \"Desktop\"")

		if desktopIndex == -1 {
			break
		}

		add += desktopIndex
		fromRe := regexp.MustCompile("https://i\\.imgur\\.com/[A-z0-9]+(\\.jpg|\\.png)|https://i\\.redd\\.it/[A-z0-9]+(\\.jpg|.png)")
		from := fromRe.FindStringIndex(pageString[add:])
		if from[0] == -1 {
			break
		}
		wg.Add(1)
		go func(from []int, add int) {
			defer wg.Done()
			var imgurl string

			imgurl = pageString[from[0]+add : from[1]+add]
			if convertLinkToImg(imgurl) != nil {
				count++
			}
		}(from, add)

		add += from[1]

	}
	wg.Wait()
	imagesAdded <- count
	close(imagesAdded)
}

func convertLinkToImg(imgurl string) error {
	var p string

	from := regexp.MustCompile("(\\.com/)|(\\.it/)").FindStringIndex(imgurl)
	p = path.Join(basePath, imgurl[from[1]+1:])
	if _, ferr := os.Stat(p); os.IsNotExist(ferr) {
		res, err := http.Get(imgurl)
		check(err, "IMGUR")
		defer res.Body.Close()
		file, err := os.Create(p)
		defer file.Close()
		check(err)

		io.Copy(file, res.Body)
		res.Body.Close()
		fmt.Println("Adding:", imgurl)
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
	req.Header.Set("User-Agent", "Wallpaper-bot-0.2")
	return client.Do(req)

}

func setDesktop(file string) {
	file = path.Join(workingPath, basePath, file)
	fileptr, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		log.Fatal(err)
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
	return files[ran].Name()
}

func getInput(output string) (string, error) {
	fmt.Print(output)
	return console.ReadString('\n')
}

func check(err error, outs ...string) {
	if err != nil {
		for _, s := range outs {
			log.Println(s)
		}
		log.Fatal(err)
	}
}
