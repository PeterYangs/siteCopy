package main

import "github.com/PeterYangs/siteCopy"

func main() {

	c := siteCopy.NewCopy()

	c.Url("https://www.925g.com/").Get("index.html")
}
