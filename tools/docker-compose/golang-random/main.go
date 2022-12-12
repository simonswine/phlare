package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	_ "net/http/pprof"
)

var (
	size   = 90000
	intArr = make([]int, size)
)

func doRandomStuff() {
	if (time.Now().Minute() % 5) == 0 {
		intArr = make([]int, size)
	}
	for i := 0; i < len(intArr); i++ {
		intArr[i] = rand.Int()
	}
	fmt.Println("Done randomStuff")
}

func main() {
	runtime.SetMutexProfileFraction(5)

	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	ticker := time.NewTicker(100 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			go doRandomStuff()
		}
	}
}
