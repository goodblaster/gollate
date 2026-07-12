//go:build darwin

package apple

type Engine struct{}

func (engine *Engine) String() string {
	return "apple"
}
