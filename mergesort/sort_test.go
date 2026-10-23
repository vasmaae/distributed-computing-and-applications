package mergesort_test

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/vasmaae/distributed-computing-and-applications/mergesort"
)

func TestSort(t *testing.T) {
	cases := [][]int{
		nil, {}, {42}, {2, 1}, {3, 2, 6, 9, 2, 3, 5, 8, 2, 1, 11},
		{1, 2, 3, 4, 5}, {5, 4, 3, 2, 1}, {2, 2, 2, 2}, {-5, 0, -1, 2, -5},
	}
	random := rand.New(rand.NewPCG(1, 2))
	for size := 0; size < 130; size++ {
		values := make([]int, size)
		for i := range values {
			values[i] = random.IntN(21) - 10
		}
		cases = append(cases, values)
	}
	for _, input := range cases {
		for _, threads := range []int{1, 2, 3, 5, 8, len(input) + 1} {
			for _, descending := range []bool{false, true} {
				compare := cmp.Compare[int]
				if descending {
					compare = func(a, b int) int { return cmp.Compare(b, a) }
				}
				want := slices.Clone(input)
				slices.SortFunc(want, compare)
				got := slices.Clone(input)
				if err := mergesort.Sort(got, threads, compare); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, want) {
					t.Fatalf("input=%v threads=%d descending=%t: got %v, want %v", input, threads, descending, got, want)
				}
			}
		}
	}
}

func TestSortValidation(t *testing.T) {
	for _, threads := range []int{0, -1} {
		values := []int{2, 1}
		if err := mergesort.Sort(values, threads, cmp.Compare[int]); err == nil {
			t.Fatalf("threads=%d: expected an error", threads)
		}
		if !slices.Equal(values, []int{2, 1}) {
			t.Fatal("invalid call changed input")
		}
	}
	if err := mergesort.Sort([]int{2, 1}, 1, nil); err == nil {
		t.Fatal("nil compare: expected an error")
	}
}

func TestSortRecords(t *testing.T) {
	type record struct{ Key, ID int }
	input := []record{{2, 1}, {1, 2}, {2, 3}, {1, 4}, {2, 5}, {1, 6}, {2, 7}}
	compare := func(a, b record) int { return cmp.Compare(a.Key, b.Key) }
	want := slices.Clone(input)
	slices.SortStableFunc(want, compare)
	for _, threads := range []int{1, 2, 3, 5, 20} {
		got := slices.Clone(input)
		if err := mergesort.Sort(got, threads, compare); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("threads=%d: got %v, want %v", threads, got, want)
		}
	}
}

func BenchmarkSort(b *testing.B) {
	for _, size := range []int{1_000, 100_000, 1_000_000} {
		random := rand.New(rand.NewPCG(1, 2))
		input := make([]int, size)
		for i := range input {
			input[i] = random.Int()
		}
		for _, threads := range []int{1, 2, 3, 5, 8} {
			b.Run(fmt.Sprintf("n=%d/threads=%d", size, threads), func(b *testing.B) {
				values := make([]int, size)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					copy(values, input)
					b.StartTimer()
					if err := mergesort.Sort(values, threads, cmp.Compare[int]); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				if !slices.IsSorted(values) {
					b.Fatal("result is not sorted")
				}
			})
		}
	}
}
