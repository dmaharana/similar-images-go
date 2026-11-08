package cluster

// DisjointSet implements union-find with path compression and rank optimization.
type DisjointSet struct {
	parent []int
	rank   []int
}

// NewDisjointSet creates a new disjoint set for n elements.
func NewDisjointSet(n int) *DisjointSet {
	parent := make([]int, n)
	rank := make([]int, n)
	for i := 0; i < n; i++ {
		parent[i] = i
	}
	return &DisjointSet{
		parent: parent,
		rank:   rank,
	}
}

// Find finds the representative root of element x with path compression.
func (ds *DisjointSet) Find(x int) int {
	root := x
	for root != ds.parent[root] {
		root = ds.parent[root]
	}
	// Path compression
	curr := x
	for curr != root {
		next := ds.parent[curr]
		ds.parent[curr] = root
		curr = next
	}
	return root
}

// Union merges the sets containing x and y. Returns true if sets were merged, false if already in same set.
func (ds *DisjointSet) Union(x, y int) bool {
	rootX := ds.Find(x)
	rootY := ds.Find(y)
	if rootX == rootY {
		return false
	}

	if ds.rank[rootX] < ds.rank[rootY] {
		ds.parent[rootX] = rootY
	} else if ds.rank[rootX] > ds.rank[rootY] {
		ds.parent[rootY] = rootX
	} else {
		ds.parent[rootY] = rootX
		ds.rank[rootX]++
	}

	return true
}
