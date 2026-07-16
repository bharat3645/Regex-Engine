// File: dispatcher/priority_queue.go
package dispatcher

import (
	"Regex/types"
	"container/heap"
)

// PriorityQueue implements heap.Interface and holds FileJobs.
// It is a min-heap, but we use (PriorityHigh - job.Priority) to make it behave like a max-heap.
type PriorityQueue []*types.FileJob

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority. So we use >
	return pq[i].Priority > pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

// Push adds an element to the heap.
func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*types.FileJob)
	*pq = append(*pq, item)
}

// Pop removes and returns the highest priority element from the heap.
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*pq = old[0 : n-1]
	return item
}

// NewPriorityQueue creates a new, initialized priority queue.
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{}
	heap.Init(pq)
	return pq
}
