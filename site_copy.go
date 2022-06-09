package siteCopy

import (
	"archive/zip"
	"context"
	"fmt"
	"github.com/PeterYangs/request/v2"
	"github.com/PeterYangs/tools"
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

type SiteCopy struct {
	client       *request.Client
	downloadChan chan []string
	wait         sync.WaitGroup
	fileCollect  sync.Map
	lock         sync.Mutex
	fileIndex    int
	SiteUrlList  []*SiteUrl
	zipWriter    *zip.Writer
	cxt          context.Context
	cancel       context.CancelFunc
}

func NewCopy(cxt context.Context) *SiteCopy {

	client := request.NewClient()

	c, cancel := context.WithCancel(cxt)

	s := &SiteCopy{
		client:       client,
		downloadChan: make(chan []string, 10),
		wait:         sync.WaitGroup{},
		fileCollect:  sync.Map{},
		lock:         sync.Mutex{},
		cxt:          c,
		cancel:       cancel,
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

			err := sy.do(s[0], s[1])

			if err != nil {

				fmt.Println(err)
			}

		}

	}

}

func (sy *SiteCopy) do(link string, name string) error {

	//return nil

	sy.wait.Add(1)

	defer sy.wait.Done()

	rsp, err := sy.client.R().Get(link)

	if err != nil {

		return err
	}

	defer rsp.GetResponse().Body.Close()

	err = sy.WriteZip(name, rsp.GetResponse().Body)

	if err != nil {

		return err
	}

	sy.fileCollect.Store(link, name)

	return nil

}

func (sy *SiteCopy) WriteZip(name string, body io.Reader) error {

	sy.lock.Lock()

	defer sy.lock.Unlock()

	w, err := sy.zipWriter.Create(name)

	if err != nil {

		return err
	}

	_, err = io.Copy(w, body)

	if err != nil {

		return err
	}

	return nil
}

func (sy *SiteCopy) push(u string, fileType FileType) string {

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

	arr := []string{u, filename}

	sy.downloadChan <- arr

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

				filename := sl.SiteCopy.push(sl.getLink(v), CSS)

				selection.SetAttr("href", filename)

			}

		})

		doc.Find("script").Each(func(i int, selection *goquery.Selection) {

			v, ok := selection.Attr("src")

			if ok && v != "" {

				filename := sl.SiteCopy.push(sl.getLink(v), JS)

				selection.SetAttr("src", filename)

			}

		})

		doc.Find("img").Each(func(i int, selection *goquery.Selection) {

			v, ok := selection.Attr("src")

			if ok && v != "" {

				filename := sl.SiteCopy.push(sl.getLink(v), IMAGE)

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

		err = sy.WriteZip(sl.name, strings.NewReader(html))

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
