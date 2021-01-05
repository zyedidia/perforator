package main

import (
	"fmt"
	"math/rand"
	"runtime"
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
	numbers := make([]int32, size)
	for i := 0; i < size; i++ {
		numbers[i] = rand.Int31()
	}

	s := sum(numbers)
	fmt.Println(s)
}
