/*
Copyright 2026 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package rulesets provides rulesets related operations.
package rulesets

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/coreruleset/seclang_parser/parser"
)

var knownOperators map[string]bool

func init() {
	knownOperators = map[string]bool{
		"beginsWith":           true,
		"contains":             true,
		"detectSQLi":           true,
		"detectXSS":            true,
		"endsWith":             true,
		"eq":                   true,
		"ge":                   true,
		"gt":                   true,
		"inspectFile":          true,
		"ipMatch":              true,
		"le":                   true,
		"lt":                   true,
		"noMatch":              true,
		"pm":                   true,
		"rbl":                  true,
		"restpath":             true,
		"rx":                   true,
		"streq":                true,
		"unconditionalMatch":   true,
		"validateByteRange":    true,
		"validateNid":          true,
		"validateSchema":       true,
		"validateURLEncoding":  true,
		"validateUtf8Encoding": true,
		"within":               true,
	}
}

func isValid(hay map[string]bool, needle string) bool {
	supported, exists := hay[needle]
	return exists && supported
}

func isValidCollection(collection string) bool {
	return true
}

func isValidOperator(operator string) bool {
	return isValid(knownOperators, operator)
}

func isValidSetvarCollection(setvarCollection string) bool {
	return true
}

func isValidVariable(variable string) bool {
	return true
}

type position struct {
	Line   int
	Column int
}

func appendPos(target map[string][]position, name string, line, column int) {
	target[name] = append(target[name], position{line, column})
}

type violations struct {
	Variables         map[string][]position
	Collections       map[string][]position
	Operators         map[string][]position
	SetvarCollections map[string][]position
}

func (v *violations) errors() []error {
	errs := make([]error, 0)
	for directive, positions := range v.Operators {
		for _, pos := range positions {
			errs = append(errs, fmt.Errorf("[%d:%d] Unsupported operator: @%s", pos.Line, pos.Column, directive))
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type validationListener struct {
	*parser.BaseSecLangParserListener
	violations violations
}

func newValidationListener() *validationListener {
	listener := new(validationListener)
	listener.violations.Operators = make(map[string][]position)
	return listener
}

type customErrorListener struct {
	*antlr.DefaultErrorListener
	Errors []error
}

func newCustomErrorListener() *customErrorListener {
	return &customErrorListener{antlr.NewDefaultErrorListener(), make([]error, 0)}
}

// SyntaxError records an error
func (c *customErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	var err error
	if offendingSymbol == nil {
		err = fmt.Errorf("[%d:%d] Recognition error: %s", line, column, msg)
	} else {
		err = fmt.Errorf("[%d:%d] Syntax error '%v': %s", line, column, offendingSymbol, msg)
	}
	c.Errors = append(c.Errors, err)
}

// nolint
func (t *validationListener) EnterEveryRule(ctx antlr.ParserRuleContext) {
	// if you need to debug, enable this one below
	// fmt.Println(ctx.GetText())
}

// nolint
func (l *validationListener) EnterVariable_enum(ctx *parser.Variable_enumContext) {
	variable := ctx.GetText()
	if !isValidVariable(variable) {
		appendPos(l.violations.Variables, variable, ctx.GetStart().GetLine(), ctx.GetStart().GetColumn())
	}
}

// nolint
func (l *validationListener) EnterCollection_enum(ctx *parser.Collection_enumContext) {
	collection := ctx.GetText()
	if !isValidCollection(collection) {
		appendPos(l.violations.Collections, collection, ctx.GetStart().GetLine(), ctx.GetStart().GetColumn())
	}
}

// nolint
func (l *validationListener) EnterOperator_name(ctx *parser.Operator_nameContext) {
	operator := ctx.GetText()
	if !isValidOperator(operator) {
		appendPos(l.violations.Operators, operator, ctx.GetStart().GetLine(), ctx.GetStart().GetColumn())
	}
}

// nolint
func (l *validationListener) EnterCol_name(ctx *parser.Col_nameContext) {
	setvarCollection := ctx.GetText()
	if !isValidSetvarCollection(setvarCollection) {
		appendPos(l.violations.SetvarCollections, setvarCollection, ctx.GetStart().GetLine(), ctx.GetStart().GetColumn())
	}
}

// Validate parses SecRules for being valid and only uses supported features
// If there are any violation, they are returned a []error
func Validate(seclang string) []error {
	input := antlr.NewInputStream(seclang)
	lexer := parser.NewSecLangLexer(input)

	lexer.RemoveErrorListeners()

	parserErrors := newCustomErrorListener()
	stream := antlr.NewCommonTokenStream(lexer, 0)
	p := parser.NewSecLangParser(stream)
	p.RemoveErrorListeners()
	p.AddErrorListener(parserErrors)
	p.BuildParseTrees = true
	tree := p.Configuration()

	validationErrors := newValidationListener()
	antlr.ParseTreeWalkerDefault.Walk(validationErrors, tree)

	if len(parserErrors.Errors) > 0 {
		return parserErrors.Errors
	}

	return validationErrors.violations.errors()
}
