// Copyright ©2012 The bíogo.kdtree Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package kdtree implements a k-d tree.
package kdtree

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
)

type Interface interface {
	// Index returns the ith element of the list of points.
	Index(i int) Comparable

	// Len returns the length of the list.
	Len() int

	// Pivot partitions the list based on the dimension specified.
	Pivot(Dim) int

	// Slice returns a slice of the list.
	Slice(start, end int) Interface
}

// An Bounder returns a bounding volume containing the list of points. Bounds may return nil.
type Bounder interface {
	Bounds() *Bounding
}

type bounder interface {
	Interface
	Bounder
}

// A Dim is an index into a point's coordinates.
type Dim int

// A Comparable is the element interface for values stored in a k-d tree.
type Comparable interface {
	// Clone returns a copy of the Comparable.
	Clone() Comparable

	// Compare returns the shortest translation of the plane through b with
	// normal vector along dimension d to the parallel plane through a.
	//
	// Given c = a.Compare(b, d):
	//  c = a_d - b_d
	//
	Compare(Comparable, Dim) float64

	// Dims returns the number of dimensions described in the Comparable.
	Dims() int

	// Distance returns the squared Euclidian distance between the receiver and
	// the parameter.
	Distance(Comparable) float64
}

// An Extender can increase a bounding volume to include the point. Extend may return nil.
type Extender interface {
	Extend(*Bounding) *Bounding
}

type extender interface {
	Comparable
	Extender
}

// A Bounding represents a volume bounding box.
type Bounding [2]Comparable

// Contains returns whether c is within the volume of the Bounding. A nil Bounding
// returns true.
func (b *Bounding) Contains(c Comparable) bool {
	if b == nil {
		return true
	}
	for d := Dim(0); d < Dim(c.Dims()); d++ {
		if c.Compare(b[0], d) < 0 || c.Compare(b[1], d) > 0 {
			return false
		}
	}
	return true
}

// A Node holds a single point value in a k-d tree.
type Node struct {
	Point       Comparable
	Plane       Dim
	Left, Right *Node
	*Bounding
}

func (n *Node) String() string {
	if n == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%.3f %d", n.Point, n.Plane)
}

// A Tree implements a k-d tree creation and nearest neighbour search.
type Tree struct {
	Root  *Node
	Count int
}

// New returns a k-d tree constructed from the values in p. If p is a Bounder and
// bounding is true, bounds are determined for each node.
func New(p Interface, bounding bool) *Tree {
	if p, ok := p.(bounder); ok && bounding {
		return &Tree{
			Root:  buildBounded(p, 0, bounding),
			Count: p.Len(),
		}
	}
	return &Tree{
		Root:  build(p, 0),
		Count: p.Len(),
	}
}

func build(p Interface, plane Dim) *Node {
	if p.Len() == 0 {
		return nil
	}

	piv := p.Pivot(plane)
	d := p.Index(piv)
	np := (plane + 1) % Dim(d.Dims())

	return &Node{
		Point:    d,
		Plane:    plane,
		Left:     build(p.Slice(0, piv), np),
		Right:    build(p.Slice(piv+1, p.Len()), np),
		Bounding: nil,
	}
}

func buildBounded(p bounder, plane Dim, bounding bool) *Node {
	if p.Len() == 0 {
		return nil
	}

	piv := p.Pivot(plane)
	d := p.Index(piv)
	np := (plane + 1) % Dim(d.Dims())

	var b *Bounding
	if bounding {
		b = p.Bounds()
	}
	return &Node{
		Point:    d,
		Plane:    plane,
		Left:     buildBounded(p.Slice(0, piv).(bounder), np, bounding),
		Right:    buildBounded(p.Slice(piv+1, p.Len()).(bounder), np, bounding),
		Bounding: b,
	}
}

// Insert adds a point to the tree, updating the bounding volumes if bounding is
// true, and the tree is empty or the tree already has bounding volumes stored,
// and c is an Extender. No rebalancing of the tree is performed.
func (t *Tree) Insert(c Comparable, bounding bool) {
	t.Count++
	if t.Root != nil {
		bounding = t.Root.Bounding != nil
	}
	if c, ok := c.(extender); ok && bounding {
		t.Root = t.Root.insertBounded(c, 0, bounding)
		return
	} else if !ok && t.Root != nil {
		// If we are not rebounding, mark the tree as non-bounded.
		t.Root.Bounding = nil
	}
	t.Root = t.Root.insert(c, 0)
}

func (n *Node) insert(c Comparable, d Dim) *Node {
	if n == nil {
		return &Node{
			Point:    c,
			Plane:    d,
			Bounding: nil,
		}
	}

	d = (n.Plane + 1) % Dim(c.Dims())
	if c.Compare(n.Point, n.Plane) <= 0 {
		n.Left = n.Left.insert(c, d)
	} else {
		n.Right = n.Right.insert(c, d)
	}

	return n
}

func (n *Node) insertBounded(c extender, d Dim, bounding bool) *Node {
	if n == nil {
		var b *Bounding
		if bounding {
			b = &Bounding{c.Clone(), c.Clone()}
		}
		return &Node{
			Point:    c,
			Plane:    d,
			Bounding: b,
		}
	}

	if bounding {
		n.Bounding = c.Extend(n.Bounding)
	}
	d = (n.Plane + 1) % Dim(c.Dims())
	if c.Compare(n.Point, n.Plane) <= 0 {
		n.Left = n.Left.insertBounded(c, d, bounding)
	} else {
		n.Right = n.Right.insertBounded(c, d, bounding)
	}

	return n
}

// Len returns the number of elements in the tree.
func (t *Tree) Len() int { return t.Count }

// Contains returns whether a Comparable is in the bounds of the tree. If no bounding has
// been constructed Contains returns true.
func (t *Tree) Contains(c Comparable) bool {
	if t.Root.Bounding == nil {
		return true
	}
	return t.Root.Contains(c)
}

var inf = math.Inf(1)

// Nearest returns the nearest value to the query and the distance between them.
func (t *Tree) Nearest(q Comparable) (Comparable, float64) {
	if t.Root == nil {
		return nil, inf
	}
	n, dist := t.Root.search(q, inf)
	if n == nil {
		return nil, inf
	}
	return n.Point, dist
}

func (n *Node) search(q Comparable, dist float64) (*Node, float64) {
	if n == nil {
		return nil, inf
	}

	c := q.Compare(n.Point, n.Plane)
	dist = math.Min(dist, q.Distance(n.Point))

	bn := n
	if c <= 0 {
		ln, ld := n.Left.search(q, dist)
		if ld < dist {
			dist = ld
			bn = ln
		}
		if c*c <= dist {
			rn, rd := n.Right.search(q, dist)
			if rd < dist {
				bn, dist = rn, rd
			}
		}
		return bn, dist
	}
	rn, rd := n.Right.search(q, dist)
	if rd < dist {
		dist = rd
		bn = rn
	}
	if c*c <= dist {
		ln, ld := n.Left.search(q, dist)
		if ld < dist {
			bn, dist = ln, ld
		}
	}
	return bn, dist
}

type NodeDist struct {
	*Node
	Dist float64
}

type nDists []NodeDist

func newNDists(n int) nDists {
	nd := make(nDists, 1, n)
	nd[0].Dist = inf
	return nd
}

func (nd *nDists) Head() NodeDist { return (*nd)[0] }
func (nd *nDists) Keep(n NodeDist) {
	if n.Dist < (*nd)[0].Dist {
		if len(*nd) == cap(*nd) {
			heap.Pop(nd)
		}
		heap.Push(nd, n)
	}
}
func (nd nDists) Len() int              { return len(nd) }
func (nd nDists) Less(i, j int) bool    { return nd[i].Dist > nd[j].Dist }
func (nd nDists) Swap(i, j int)         { nd[i], nd[j] = nd[j], nd[i] }
func (nd *nDists) Push(x interface{})   { (*nd) = append(*nd, x.(NodeDist)) }
func (nd *nDists) Pop() (i interface{}) { i, *nd = (*nd)[len(*nd)-1], (*nd)[:len(*nd)-1]; return i }

// NearestN returns the nearest n values to the query and the distances between them and the query.
func (t *Tree) NearestN(n int, q Comparable) ([]Comparable, []float64) {
	if t.Root == nil {
		return nil, []float64{inf}
	}
	nd := t.Root.searchN(q, newNDists(n))
	if len(nd) == 1 {
		if nd[0].Node == nil {
			return nil, []float64{inf}
		} else {
			return []Comparable{nd[0].Node.Point}, []float64{nd[0].Dist}
		}
	}
	sort.Sort(nd)
	for i, j := 0, len(nd)-1; i < j; i, j = i+1, j-1 {
		nd[i], nd[j] = nd[j], nd[i]
	}
	ns := make([]Comparable, len(nd))
	dist := make([]float64, len(nd))
	for i, n := range nd {
		ns[i] = n.Point
		dist[i] = n.Dist
	}
	return ns, dist
}

func (n *Node) searchN(q Comparable, dists nDists) nDists {
	if n == nil {
		return dists
	}

	c := q.Compare(n.Point, n.Plane)
	dists.Keep(NodeDist{Node: n, Dist: q.Distance(n.Point)})
	if c <= 0 {
		dists = n.Left.searchN(q, dists)
		if c*c <= dists[0].Dist {
			dists = n.Right.searchN(q, dists)
		}
		return dists
	}
	dists = n.Right.searchN(q, dists)
	if c*c <= dists[0].Dist {
		dists = n.Left.searchN(q, dists)
	}
	return dists
}

// Keeper implements a conditional max heap sorted on the Dist field of the NodeDist type.
// kd search is guided by the distance stored in the max value of the heap.
type Keeper interface {
	Head() NodeDist // Head returns the maximum element of the Keeper.
	Keep(NodeDist)  // Keep conditionally pushes the provided NodeDist onto the heap.
	heap.Interface
}

type reverse struct {
	sort.Interface
}

func (r reverse) Less(i, j int) bool { return r.Interface.Less(j, i) }

// NearestSet finds the nearest values to the query accepted by the provided Keeper.
// The Keeper retains the results.
func (t *Tree) NearestSet(k Keeper, q Comparable) {
	if t.Root == nil {
		return
	}
	t.Root.searchSet(q, k)
	if k.Len() == 1 {
		return
	}
	sort.Sort(reverse{k})
	return
}

func (n *Node) searchSet(q Comparable, k Keeper) {
	if n == nil {
		return
	}

	c := q.Compare(n.Point, n.Plane)
	k.Keep(NodeDist{Node: n, Dist: q.Distance(n.Point)})
	if c <= 0 {
		n.Left.searchSet(q, k)
		if c*c <= k.Head().Dist {
			n.Right.searchSet(q, k)
		}
		return
	}
	n.Right.searchSet(q, k)
	if c*c <= k.Head().Dist {
		n.Left.searchSet(q, k)
	}
	return
}

// An Operation is a function that operates on a Comparable. The bounding volume and tree depth
// of the point is also provided. If done is returned true, the Operation is indicating that no
// further work needs to be done and so the Do function should traverse no further.
type Operation func(Comparable, *Bounding, int) (done bool)

// Do performs fn on all values stored in the tree. A boolean is returned indicating whether the
// Do traversal was interrupted by an Operation returning true. If fn alters stored values' sort
// relationships, future tree operation behaviors are undefined.
func (t *Tree) Do(fn Operation) bool {
	if t.Root == nil {
		return false
	}
	return t.Root.do(fn, 0)
}

func (n *Node) do(fn Operation, depth int) (done bool) {
	if n.Left != nil {
		done = n.Left.do(fn, depth+1)
		if done {
			return
		}
	}
	done = fn(n.Point, n.Bounding, depth)
	if done {
		return
	}
	if n.Right != nil {
		done = n.Right.do(fn, depth+1)
	}
	return
}

// DoBounded performs fn on all values stored in the tree that are within the specified bound.
// If b is nil, the result is the same as a Do. A boolean is returned indicating whether the
// DoBounded traversal was interrupted by an Operation returning true. If fn alters stored
// values' sort relationships future tree operation behaviors are undefined.
func (t *Tree) DoBounded(fn Operation, b *Bounding) bool {
	if t.Root == nil {
		return false
	}
	if b == nil {
		return t.Root.do(fn, 0)
	}
	return t.Root.doBounded(fn, b, 0)
}

func (n *Node) doBounded(fn Operation, b *Bounding, depth int) (done bool) {
	lc, hc := b[0].Compare(n.Point, n.Plane), b[1].Compare(n.Point, n.Plane)
	if lc < 0 && n.Left != nil {
		done = n.Left.doBounded(fn, b, depth+1)
		if done {
			return
		}
	}
	if b.Contains(n.Point) {
		done = fn(n.Point, b, depth)
		if done {
			return
		}
	}
	if hc > 0 && n.Right != nil {
		done = n.Right.doBounded(fn, b, depth+1)
	}
	return
}
