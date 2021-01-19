package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"
)

const size = 10000000

func sum(numbers []int32) int64 {
	var sum int64
	for _, i := range numbers {
		sum += int64(i)
	}
	return sum
}

func main() {
	runtime.LockOSThread()
	fmt.Println(os.Getpid())

	numbers := make([]int32, size)
	for i := 0; i < size; i++ {
		numbers[i] = rand.Int31()
	}

	for i := 0; i < 1000; i++ {
		s := sum(numbers)
		fmt.Println(i, s)
		time.Sleep(time.Second)
	}
}
