package siteCopy

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"github.com/PeterYangs/request/v2"
	"github.com/PeterYangs/tools"
	"github.com/PeterYangs/tools/link"
	links "github.com/PeterYangs/tools/link"
	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cast"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
)

type FileType int

const (
	CSS   FileType = 0
	JS    FileType = 1
	IMAGE FileType = 2
)

func (f FileType) String() string {

	switch f {

	case CSS:

		return "css"

	case JS:

		return "js"

	case IMAGE:

		return "image"

	}

	return ""
}

type SiteCopy struct {
	client             *request.Client
	downloadChan       chan []string
	downloadChanBackup chan []string
	wait               sync.WaitGroup
	fileCollect        sync.Map
	lock               sync.Mutex
	fileIndex          int
	SiteUrlList        []*SiteUrl
	zipWriter          *zip.Writer
	cxt                context.Context
	cancel             context.CancelFunc
}

func NewCopy(cxt context.Context) *SiteCopy {

	client := request.NewClient()

	client.Header(map[string]string{
		"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
		"Accept-Encoding":    "gzip, deflate, br",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"sec-ch-ua-platform": "\"Windows\"",
		"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.5005.63 Safari/537.36 Edg/102.0.1245.33",
	})

	c, cancel := context.WithCancel(cxt)

	s := &SiteCopy{
		client:             client,
		downloadChan:       make(chan []string, 10),
		downloadChanBackup: make(chan []string, 1),
		wait:               sync.WaitGroup{},
		fileCollect:        sync.Map{},
		lock:               sync.Mutex{},
		cxt:                c,
		cancel:             cancel,
	}

	for i := 0; i < 10; i++ {

		go s.downloadWork()
	}

	return s
}

func (sy *SiteCopy) Url(u string, name string) *SiteUrl {

	up, _ := url.Parse(u)

	sl := &SiteUrl{
		u:        u,
		SiteCopy: sy,
		host:     up.Host,
		scheme:   up.Scheme,
		name:     name,
	}

	sy.SiteUrlList = append(sy.SiteUrlList, sl)

	return sl
}

func (sy *SiteCopy) downloadWork() {

	for {

		select {

		case s := <-sy.downloadChan:

			err := sy.do(s[0], s[1], s[2])

			if err != nil {

				fmt.Println(err)
			}

		case s := <-sy.downloadChanBackup:

			err := sy.do(s[0], s[1], s[2])

			if err != nil {

				fmt.Println(err)
			}

		}

	}

}

func (sy *SiteCopy) do(link string, name string, fileType string) error {

	//return nil

	sy.wait.Add(1)

	defer sy.wait.Done()

	ct, err := sy.client.R().GetToContent(link)

	if err != nil {

		return err
	}

	data := ct.ToString()

	if fileType == "css" {

		s := regexp.MustCompile(`url\((.*?)\)`).FindAllStringSubmatch(data, -1)

		if len(s) > 1 {

			cssImageArr := make(map[string]string)

			for _, i2 := range s[1:] {

				//fmt.Println(i2, "--------")

				cssImage, err := links.GetCompleteLink(link, i2[1])

				if err != nil {

					continue
				}

				cssImageArr[cssImage] = i2[1]

			}

			for downloadLink, ss := range cssImageArr {

				//fmt.Println(ss)

				sy.fileIndex++

				filename := "image/img" + cast.ToString(sy.fileIndex) + ".png"

				c, dErr := sy.client.R().GetToContent(downloadLink)

				if dErr != nil {

					fmt.Println(dErr)

					continue
				}

				data = strings.Replace(data, ss, "../"+filename, -1)

				sy.lock.Lock()
				w, ee := sy.zipWriter.Create(filename)

				if ee != nil {

					fmt.Println(ee)

					sy.lock.Unlock()

					continue
				}

				_, ee = io.Copy(w, bytes.NewReader(c.ToByte()))

				sy.lock.Unlock()

				//lockLink := sy.push(downloadLink, IMAGE, true)
				//
				////fmt.Println(realLink, lockLink, data)
				//
				////panic("")
				//
				//data = strings.Replace(data, realLink, lockLink, -1)
				//
				////fmt.Println(realLink, data)
				//
				//file.Write("1.txt", []byte(realLink+data))
				//
				//panic("")

			}

		}

	}

	err = sy.WriteZip(name, []byte(data))

	if err != nil {

		return err
	}

	sy.fileCollect.Store(link, name)

	return nil

}

func (sy *SiteCopy) WriteZip(name string, content []byte) error {

	sy.lock.Lock()

	defer sy.lock.Unlock()

	w, err := sy.zipWriter.Create(name)

	if err != nil {

		return err
	}

	_, err = io.Copy(w, bytes.NewReader(content))

	if err != nil {

		return err
	}

	return nil
}

func (sy *SiteCopy) push(u string, fileType FileType, isBackup bool) string {

	select {
	case <-sy.cxt.Done():

		return ""

	default:

	}

	f, ok := sy.fileCollect.Load(u)

	if ok {

		return f.(string)
	}

	sy.fileIndex++

	filename := "css/style" + cast.ToString(sy.fileIndex) + ".css"

	switch fileType {

	case CSS:

		filename = "css/style" + cast.ToString(sy.fileIndex) + ".css"

	case JS:

		filename = "js/script" + cast.ToString(sy.fileIndex) + ".js"

	case IMAGE:

		filename = "image/img" + cast.ToString(sy.fileIndex) + ".png"

	}

	arr := []string{u, filename, fileType.String()}

	if isBackup {

		sy.downloadChanBackup <- arr

	} else {

		sy.downloadChan <- arr
	}

	return filename

}

func (sy *SiteCopy) Zip(name string) error {

	archive, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)

	if err != nil {

		return err

	}

	zipWriter := zip.NewWriter(archive)

	//defer zipWriter.Close()

	sy.zipWriter = zipWriter

	defer zipWriter.Close()

	for _, sl := range sy.SiteUrlList {

		ct, err := sl.SiteCopy.client.R().GetToContent(sl.u)

		if err != nil {

			return err
		}

		html := ct.ToString()

		html, dErr := sl.dealCoding(html, ct.Header())

		if dErr != nil {

			return dErr
		}

		doc, gErr := goquery.NewDocumentFromReader(strings.NewReader(html))

		if gErr != nil {

			return gErr
		}

		doc.Find("link").Each(func(i int, selection *goquery.Selection) {

			v, ok := selection.Attr("href")

			if ok {

				sss, _ := link.GetCompleteLink(sl.u, v)

				filename := sl.SiteCopy.push(sss, CSS, false)

				selection.SetAttr("href", filename)

			}

		})

		doc.Find("script").Each(func(i int, selection *goquery.Selection) {

			v, ok := selection.Attr("src")

			if ok && v != "" {

				sss, _ := link.GetCompleteLink(sl.u, v)

				filename := sl.SiteCopy.push(sss, JS, false)

				selection.SetAttr("src", filename)

			}

		})

		doc.Find("img").Each(func(i int, selection *goquery.Selection) {

			v, ok := selection.Attr("src")

			if ok && v != "" {

				sss, _ := link.GetCompleteLink(sl.u, v)

				filename := sl.SiteCopy.push(sss, IMAGE, false)

				selection.SetAttr("src", filename)

			}

		})

		//修改编码为utf8
		doc.Find("meta").Each(func(i int, selection *goquery.Selection) {

			v, ok := selection.Attr("charset")

			if ok && v != "" {

				selection.SetAttr("charset", "utf-8")

			}

		})

		html, hErr := doc.Html()

		if hErr != nil {

			return hErr
		}

		err = sy.WriteZip(sl.name, []byte(html))

		if err != nil {

			return err
		}

		sl.SiteCopy.wait.Wait()

	}

	return nil
}

type SiteUrl struct {
	SiteCopy *SiteCopy
	u        string //原链接
	host     string
	scheme   string
	name     string
}

//----------------------------------------------------------------

// GetLink 获取完整链接
func (sl *SiteUrl) getLink(href string) string {

	case1, _ := regexp.MatchString("^/[a-zA-Z0-9_]+.*", href)

	case2, _ := regexp.MatchString("^//[a-zA-Z0-9_]+.*", href)

	case3, _ := regexp.MatchString("^(http|https).*", href)

	switch true {

	case case1:

		href = sl.scheme + "://" + sl.host + href

		break

	case case2:

		//获取当前网址的协议
		//res := regexp.MustCompile("^(https|http).*").FindStringSubmatch(sl.host)

		href = sl.scheme + "://" + sl.host + href

		break

	case case3:

		break

	default:

		href = sl.scheme + "://" + sl.host + "/" + href
	}

	return href

}

// DealCoding 解决编码问题
func (sl *SiteUrl) dealCoding(html string, header http.Header) (string, error) {

	//return html, nil

	headerContentType_ := header["Content-Type"]

	if len(headerContentType_) > 0 {

		headerContentType := headerContentType_[0]

		charset := sl.getCharsetByContentType(headerContentType)

		charset = strings.ToLower(charset)

		switch charset {

		case "gbk":

			return string(tools.ConvertToByte(html, "gbk", "utf8")), nil

		case "gb2312":

			return string(tools.ConvertToByte(html, "gbk", "utf8")), nil

		case "utf-8":

			return html, nil

		case "utf8":

			return html, nil

		case "euc-jp":

			return string(tools.ConvertToByte(html, "euc-jp", "utf8")), nil

		case "":

			break

		default:
			return string(tools.ConvertToByte(html, charset, "utf8")), nil

		}

	}

	code, err := goquery.NewDocumentFromReader(strings.NewReader(html))

	if err != nil {

		return html, err
	}

	contentType, _ := code.Find("meta[charset]").Attr("charset")

	//转小写
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	switch contentType {

	case "gbk":

		return string(tools.ConvertToByte(html, "gbk", "utf8")), nil

	case "gb2312":

		return string(tools.ConvertToByte(html, "gbk", "utf8")), nil

	case "utf-8":

		return html, nil

	case "utf8":

		return html, nil

	case "euc-jp":

		return string(tools.ConvertToByte(html, "euc-jp", "utf8")), nil

	case "":

		break
	default:
		return string(tools.ConvertToByte(html, contentType, "utf8")), nil

	}

	contentType, _ = code.Find("meta[http-equiv=\"Content-Type\"]").Attr("content")

	charset := sl.getCharsetByContentType(contentType)

	switch charset {

	case "utf-8":

		return html, nil

	case "utf8":

		return html, nil

	case "gbk":

		return string(tools.ConvertToByte(html, "gbk", "utf8")), nil

	case "gb2312":

		return string(tools.ConvertToByte(html, "gbk", "utf8")), nil

	case "euc-jp":

		return string(tools.ConvertToByte(html, "euc-jp", "utf8")), nil

	case "":

		break

	default:
		return string(tools.ConvertToByte(html, charset, "utf8")), nil

	}

	return html, nil
}

// GetCharsetByContentType 从contentType中获取编码
func (sl *SiteUrl) getCharsetByContentType(contentType string) string {

	contentType = strings.TrimSpace(strings.ToLower(contentType))

	//捕获编码
	r, _ := regexp.Compile(`charset=([^;]+)`)

	re := r.FindAllStringSubmatch(contentType, 1)

	if len(re) > 0 {

		c := re[0][1]

		return c

	}

	return ""
}
