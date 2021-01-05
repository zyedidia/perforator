#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <stdint.h>

#define SIZE 10000000

// Perforator requires that benchmark functions are not inlined.
uint64_t __attribute__ ((noinline)) sum(uint32_t* numbers) {
    uint64_t sum = 0;
    for (int i = 0; i < SIZE; i++) {
        sum += numbers[i];
    }
    return sum;
}

int main() {
    srand(time(NULL));
    uint32_t* numbers = malloc(SIZE * sizeof(uint32_t));
    for (int i = 0; i < SIZE; i++) {
        numbers[i] = rand();
    }

    uint64_t result = sum(numbers);
    printf("%lu\n", result);
    return 0;
}
