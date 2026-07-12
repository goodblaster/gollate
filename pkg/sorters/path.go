package sorters

import (
	"strings"
)

type Path struct {
	Length float64
	Nodes  []int
}

func (path *Path) Contains(index int) bool {
	for _, i := range path.Nodes {
		if i == index {
			return true
		}
	}
	return false
}

func (path *Path) Copy() Path {
	nodes := make([]int, len(path.Nodes))
	copy(nodes, path.Nodes)
	return Path{
		Length: path.Length,
		Nodes:  nodes,
	}
}

func (path *Path) Append(word Block, distance float64) {
	path.Nodes = append(path.Nodes, word.Index)
	path.Length += distance
}

// AppendHole records a bridged wildcard slot (EnableChainHoles): no block is
// consumed and the hole penalty is added to the path length.
func (path *Path) AppendHole(penalty float64) {
	path.Nodes = append(path.Nodes, HoleNode)
	path.Length += penalty
}

func (path *Path) String(block []Block) string {
	var words []string
	for _, node := range path.Nodes {
		if node < 0 {
			words = append(words, "<hole>")
			continue
		}
		words = append(words, block[node].NormedText)
	}
	return strings.Join(words, " ")
}
