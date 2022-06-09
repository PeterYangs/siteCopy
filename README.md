# site-copy

仿站工具

## 安装
```shell
go get github.com/PeterYangs/siteCopy
```

## 快速开始
```go
package main

import (
	"context"
	"fmt"
	"github.com/PeterYangs/siteCopy"
	"time"
)

func main() {

	cxt, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer cancel()

	c := siteCopy.NewCopy(cxt)

	c.Url("https://www.925g.com/", "index.html")

	c.Url("https://www.925g.com/zixun/", "news.html")

	err := c.Zip("archive.zip")

	if err != nil {

		fmt.Println(err)
	}
}

```