# go-future

A concurrency golang library to support async tasks with using go routines 

## Install

```sh
$ go get github.com/hjyun328/go-future
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/hjyun328/go-future/future"
    "time"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
    defer cancel()

    result, err := future.All(
       ctx,
       future.New(func() (string, error) {
           time.Sleep(3 * time.Second)
           return "A", nil
       }),
       future.New(func() (string, error) {
           time.Sleep(2 * time.Second)
           return "B", nil
       }),
       future.New(func() (string, error) {
           time.Sleep(1 * time.Second)
           return "C", nil
       }),
    ).Await(ctx)
    if err != nil {
       panic(err)
    }

    fmt.Println(result) // [A B C]
}
```
