package siteCopy

import (
	"github.com/PeterYangs/request/v2"
	"github.com/PeterYangs/tools"
	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cast"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
)

type SiteCopy struct {
	client       *request.Client
	downloadChan chan []string
	wait         sync.WaitGroup
}

func NewCopy() *SiteCopy {

	client := request.NewClient()

	s := &SiteCopy{
		client:       client,
		downloadChan: make(chan []string, 10),
		wait:         sync.WaitGroup{},
	}

	for i := 0; i < 10; i++ {

		go s.downloadWork()
	}

	return s
}

func (sy *SiteCopy) Url(u string) *SiteUrl {

	up, _ := url.Parse(u)

	return &SiteUrl{
		u:        u,
		SiteCopy: sy,
		host:     up.Host,
		scheme:   up.Scheme,
	}
}

func (sy *SiteCopy) downloadWork() {

	for {

		select {

		case s := <-sy.downloadChan:

			sy.wait.Add(1)

			sy.client.R().Download(s[0], s[1])

			sy.wait.Done()

		}

	}

}

func (sy *SiteCopy) push(u string, path string) {

	arr := []string{u, path}

	sy.downloadChan <- arr

}

type SiteUrl struct {
	SiteCopy *SiteCopy
	u        string //原链接
	host     string
	scheme   string
}

func (sl *SiteUrl) Get(name string) error {

	ct, err := sl.SiteCopy.client.R().GetToContent(sl.u)

	if err != nil {

		return err
	}

	html := ct.ToString()

	html, dErr := sl.dealCoding(html, ct.Header())

	if dErr != nil {

		return dErr
	}

	//fmt.Println(html)

	doc, gErr := goquery.NewDocumentFromReader(strings.NewReader(html))

	if gErr != nil {

		return gErr
	}

	os.MkdirAll("css", 0755)

	doc.Find("link").Each(func(i int, selection *goquery.Selection) {

		v, ok := selection.Attr("href")

		if ok {

			//sl.SiteCopy.client.R().Download(sl.getLink(v), "css/style"+cast.ToString(i)+".css")

			sl.SiteCopy.push(sl.getLink(v), "css/style"+cast.ToString(i)+".css")

			selection.SetAttr("href", "css/style"+cast.ToString(i)+".css")

		}

	})

	os.MkdirAll("js", 0755)

	doc.Find("script").Each(func(i int, selection *goquery.Selection) {

		v, ok := selection.Attr("src")

		if ok && v != "" {

			//sl.SiteCopy.client.R().Download(sl.getLink(v), "js/script"+cast.ToString(i)+".js")

			sl.SiteCopy.push(sl.getLink(v), "js/script"+cast.ToString(i)+".js")

			selection.SetAttr("src", "js/script"+cast.ToString(i)+".js")

		}

	})

	os.MkdirAll("image", 0755)

	doc.Find("img").Each(func(i int, selection *goquery.Selection) {

		v, ok := selection.Attr("src")

		if ok && v != "" {

			//sl.SiteCopy.client.R().Download(sl.getLink(v), "image/img"+cast.ToString(i)+".png")

			sl.SiteCopy.push(sl.getLink(v), "image/img"+cast.ToString(i)+".png")

			selection.SetAttr("src", "image/img"+cast.ToString(i)+".png")

		}

	})

	html, hErr := doc.Html()

	if hErr != nil {

		return hErr
	}

	f, fErr := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)

	if fErr != nil {

		return fErr
	}

	f.Write([]byte(html))

	sl.SiteCopy.wait.Wait()

	return nil

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
