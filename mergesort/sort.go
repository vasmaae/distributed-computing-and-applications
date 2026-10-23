// Package mergesort implements parallel merge sort with a bounded number of goroutines.
package mergesort

import (
	"errors"
	"sync"
)

// Sort sorts values in place using compare, which follows the slices.SortFunc convention.
// Equal elements retain their original order. compare must be safe for concurrent calls.
// threads must be positive; at most min(threads, len(values)) goroutines work at once.
func Sort[T any](values []T, threads int, compare func(T, T) int) error {
	if threads <= 0 {
		return errors.New("threads must be positive")
	}
	if compare == nil {
		return errors.New("compare must not be nil")
	}
	if len(values) < 2 {
		return nil
	}

	parts := min(threads, len(values))
	buffer := make([]T, len(values))
	boundaries := make([]int, parts+1)
	for i := range boundaries {
		boundaries[i] = i * len(values) / parts
	}

	var workers sync.WaitGroup
	for i := 0; i < parts; i++ {
		start, end := boundaries[i], boundaries[i+1]
		workers.Add(1)
		go func() {
			defer workers.Done()
			sort(values[start:end], buffer[start:end], compare)
		}()
	}
	workers.Wait()

	source, target := values, buffer
	for parts > 1 {
		for i := 0; i < parts; i += 2 {
			start, middle, end := boundaries[i], boundaries[i+1], boundaries[min(i+2, parts)]
			workers.Add(1)
			go func() {
				defer workers.Done()
				merge(target[start:end], source[start:middle], source[middle:end], compare)
			}()
		}
		workers.Wait()
		for i := 0; i <= (parts+1)/2; i++ {
			boundaries[i] = boundaries[min(2*i, parts)]
		}
		parts = (parts + 1) / 2
		source, target = target, source
	}
	if &source[0] != &values[0] {
		copy(values, source)
	}
	return nil
}

func sort[T any](values, buffer []T, compare func(T, T) int) {
	if len(values) < 2 {
		return
	}
	middle := len(values) / 2
	sort(values[:middle], buffer[:middle], compare)
	sort(values[middle:], buffer[middle:], compare)
	merge(buffer, values[:middle], values[middle:], compare)
	copy(values, buffer)
}

func merge[T any](target, left, right []T, compare func(T, T) int) {
	i, j, k := 0, 0, 0
	for i < len(left) && j < len(right) {
		if compare(left[i], right[j]) <= 0 {
			target[k] = left[i]
			i++
		} else {
			target[k] = right[j]
			j++
		}
		k++
	}
	k += copy(target[k:], left[i:])
	copy(target[k:], right[j:])
}
