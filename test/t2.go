package main

import (
	"fmt"
	"github.com/PeterYangs/request/v2"
)

func main() {

	c := request.NewClient()

	err := c.R().Download("https://www.925g.com/", "index2.html")

	if err != nil {

		fmt.Println(err)

		return
	}

	//fmt.Println(ct.ToString())

}
