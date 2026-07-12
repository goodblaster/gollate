package sorters

type NextIndex struct {
	i int
}

func (next *NextIndex) Index() int {
	defer func() { next.i++ }()
	return next.i
}
