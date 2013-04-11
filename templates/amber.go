package templates

import (
	"github.com/PuerkitoBio/ghost"
	"github.com/eknkc/amber"
)

// The template compiler for Amber templates.
type AmberCompiler struct {
	Options amber.Options
	c       *amber.Compiler
}

// Create a new Amber compiler with the specified Amber-specific options.
func NewAmberCompiler(opts amber.Options) *AmberCompiler {
	return &AmberCompiler{
		opts,
		nil,
	}
}

// Implementation of the TemplateCompiler interface.
func (this *AmberCompiler) Compile(f string) (ghost.Templater, error) {
	// amber.CompileFile creates a new compiler each time. To limit the number
	// of allocations, reuse a compiler.
	if this.c == nil {
		this.c = amber.New()
	}
	this.c.Options = this.Options
	if err := this.c.ParseFile(f); err != nil {
		return nil, err
	}
	return this.c.Compile()
}
