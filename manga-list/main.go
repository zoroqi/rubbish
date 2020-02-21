package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/anaskhan96/soup"
	"github.com/valyala/fasthttp"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Manga struct {
	Web            string `json"web"`
	Title          string `json:"title"`
	Link           string `json:"link"`
	Type           string `json:"type"`
	LastUpdateTime string `json:"lastUpdateTime"`
	Source         string `json:"source"`
}

func main() {

	rand.Seed(time.Now().UnixNano())
	f := flag.String("f", "", "output file")
	pageRange := flag.String("p", "1-5", "page range, \\d-\\d")
	flag.Parse()

	if *f == "" {
		fmt.Println("f empty")
		return
	}

	if *pageRange == "" {
		*pageRange = "1-5"
	}

	file, err := os.OpenFile(*f, os.O_CREATE|os.O_RDWR|os.O_APPEND|os.O_SYNC, 0666)
	if err != nil {
		fmt.Println("open file err", err)
		return
	}

	pr := strings.Split(*pageRange, "-")
	if len(pr) != 2 {
		fmt.Println("page range err, ", *pageRange)
		return
	}

	start, end, err := parseRange(pr, pageRange)
	if err != nil {
		fmt.Println("page range err, ", *pageRange)
	}

	wait := sync.WaitGroup{}
	wait.Add(2)
	listResult := make(chan Manga)
	fmt.Printf("start:%d,end:%d\n", start, end)
	go writeFile(file, listResult, &wait)
	go manhuaguiSpider(start, end, listResult, &wait)
	wait.Wait()
}

func writeFile(file *os.File, listResult chan Manga, wait *sync.WaitGroup) {
	defer wait.Done()
	count := 0
	for {
		manga, c := <-listResult
		if c {
			str, err := json.Marshal(manga)
			if err != nil {
				fmt.Printf("marshal err, %+v, %v\n", manga, err)
				continue
			}
			count++
			_, err = file.WriteString(string(str) + "\n")
			if err != nil {
				fmt.Printf("write err, %+v, %v\n", manga, err)
				continue
			}
		} else {
			break
		}
	}
	fmt.Printf("write end, write:%d\n", count)
}

func manhuaguiSpider(start int, end int, listResult chan Manga, wait *sync.WaitGroup) {
	defer wait.Done()
	defer close(listResult)
	count := 0
	for i := start; i <= end; i++ {
		second := rand.Int63n(20) + 50
		time.Sleep(time.Second * time.Duration(second))
		baseUrl := "https://www.manhuagui.com/list/"
		var url string
		if i == 1 {
			url = baseUrl
		} else {
			url = baseUrl + "index_p" + strconv.Itoa(i) + ".html"
		}

		mangas, err := manhuaguiListParse(url)
		if err != nil {
			fmt.Printf("list err, %s,%+v\n", url, err)
			continue
		}
		fmt.Printf("list success, %d, mangas:%d\n", i, len(mangas))
		for _, m := range mangas {
			listResult <- m
			count++
		}
	}
	fmt.Printf("download end, download:%d\n", count)
}

func manhuaguiDownload(url string) (int, []byte, error) {
	req := &fasthttp.Request{}
	req.SetRequestURI(url)
	req.Header.SetReferer("https://www.manhuagui.com/list/")
	req.Header.SetUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.100 Safari/537.36")
	req.Header.SetMethod("GET")
	resp := &fasthttp.Response{}
	client := &fasthttp.Client{}
	if err := client.Do(req, resp); err != nil {
		return 0, nil, err
	}

	b := resp.Body()
	return resp.StatusCode(), b, nil
}

func parseRange(pr []string, pageRange *string) (int, int, error) {
	start, err := strconv.Atoi(pr[0])
	if err != nil {
		fmt.Println("page range err, ", *pageRange)
		return 0, 0, err
	}
	end, err := strconv.Atoi(pr[1])
	if err != nil {
		fmt.Println("page range err, ", *pageRange)
		return 0, 0, err
	}

	if start > end {
		fmt.Println("page range err, ", *pageRange)
		return 0, 0, errors.New("start > end")
	}
	return start, end, nil
}

func manhuaguiListParse(link string) ([]Manga, error) {
	statusCode, body, err := manhuaguiDownload(link)
	if err != nil {
		return nil, err
	}
	if statusCode != 200 {
		return nil, errors.New("code != 200")
	}

	doc := soup.HTMLParse(string(body))
	mangalist := doc.Find("ul", "id", "contList").FindAll("li")
	var manga []Manga
	for _, m := range mangalist {
		a := m.Find("p").Find("a")
		span := m.Find("span", "class", "updateon")
		fd := m.Find("span", "class", "fd")
		sl := m.Find("span", "class", "sl")
		mangaType := ""
		if fd.Error != nil {
			mangaType += "完结"
		}
		if sl.Error != nil {
			mangaType += "连载"
		}

		manga = append(manga, Manga{
			Web:            "manhuagui",
			Title:          a.Attrs()["title"],
			Link:           a.Attrs()["href"],
			Type:           mangaType,
			LastUpdateTime: span.Text(),
			Source:         link,
		})
	}
	return manga, nil
}
