package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleroot
	rulesep
	rulespaces
	ruletable_def
	ruletable_lb
	ruletable_rb
	ruletable_name
	rulecolumns
	rulecolumn
	rulecolumn_description
	ruledot
	rulecolumn_name_with_relation
	rulecolumn_name_only
	rulecolumn_name
	rulerarrow
	rulerdotarrow
	rulerlinearrow
	ruletarget_table_name
	ruletarget_column_name
	ruleEOT
	ruleAction0
	rulePegText
	ruleAction1
	ruleAction2
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8

	rulePre_
	rule_In_
	rule_Suf
)

var rul3s = [...]string{
	"Unknown",
	"root",
	"sep",
	"spaces",
	"table_def",
	"table_lb",
	"table_rb",
	"table_name",
	"columns",
	"column",
	"column_description",
	"dot",
	"column_name_with_relation",
	"column_name_only",
	"column_name",
	"rarrow",
	"rdotarrow",
	"rlinearrow",
	"target_table_name",
	"target_column_name",
	"EOT",
	"Action0",
	"PegText",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next uint32, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(string(([]rune(buffer)[node.begin:node.end]))))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next uint32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: uint32(t.begin), end: uint32(t.end), next: uint32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = uint32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, uint32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth uint32, index int) {
	t.tree[index] = token32{pegRule: rule, begin: uint32(begin), end: uint32(end), next: uint32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/*func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2 * len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}*/

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type Parser struct {
	Tables []Table
	table  *Table
	column *Column

	Buffer string
	buffer []rune
	rules  [31]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	Pretty bool
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *Parser
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *Parser) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *Parser) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *Parser) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
			text = string(_buffer[begin:end])

		case ruleAction0:

			p.Tables = append(p.Tables, *p.table)

		case ruleAction1:

			p.table = &Table{
				Name:    text,
				Columns: make([]Column, 0),
			}

		case ruleAction2:

			p.table.Columns = append(p.table.Columns, *p.column)

		case ruleAction3:

			p.column.Description = text

		case ruleAction4:

			p.column = &Column{
				Name: text,
			}

		case ruleAction5:

			p.column.Relation = &Relation{
				LineType: DotLine,
			}

		case ruleAction6:

			p.column.Relation = &Relation{
				LineType: NormalLine,
			}

		case ruleAction7:

			p.column.Relation.TableName = text

		case ruleAction8:

			p.column.Relation.ColumnName = text

		}
	}
	_, _, _, _, _ = buffer, _buffer, text, begin, end
}

func (p *Parser) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens32{tree: make([]token32, math.MaxInt16)}
	var max token32
	position, depth, tokenIndex, buffer, _rules := uint32(0), uint32(0), 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin uint32) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position, depth}
		}
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 root <- <((sep* table_def)* sep* EOT)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
			l2:
				{
					position3, tokenIndex3, depth3 := position, tokenIndex, depth
				l4:
					{
						position5, tokenIndex5, depth5 := position, tokenIndex, depth
						if !_rules[rulesep]() {
							goto l5
						}
						goto l4
					l5:
						position, tokenIndex, depth = position5, tokenIndex5, depth5
					}
					if !_rules[ruletable_def]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex, depth = position3, tokenIndex3, depth3
				}
			l6:
				{
					position7, tokenIndex7, depth7 := position, tokenIndex, depth
					if !_rules[rulesep]() {
						goto l7
					}
					goto l6
				l7:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
				}
				if !_rules[ruleEOT]() {
					goto l0
				}
				depth--
				add(ruleroot, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 sep <- <('\n' / '\t' / ' ')> */
		func() bool {
			position8, tokenIndex8, depth8 := position, tokenIndex, depth
			{
				position9 := position
				depth++
				{
					position10, tokenIndex10, depth10 := position, tokenIndex, depth
					if buffer[position] != rune('\n') {
						goto l11
					}
					position++
					goto l10
				l11:
					position, tokenIndex, depth = position10, tokenIndex10, depth10
					if buffer[position] != rune('\t') {
						goto l12
					}
					position++
					goto l10
				l12:
					position, tokenIndex, depth = position10, tokenIndex10, depth10
					if buffer[position] != rune(' ') {
						goto l8
					}
					position++
				}
			l10:
				depth--
				add(rulesep, position9)
			}
			return true
		l8:
			position, tokenIndex, depth = position8, tokenIndex8, depth8
			return false
		},
		/* 2 spaces <- <' '+> */
		func() bool {
			position13, tokenIndex13, depth13 := position, tokenIndex, depth
			{
				position14 := position
				depth++
				if buffer[position] != rune(' ') {
					goto l13
				}
				position++
			l15:
				{
					position16, tokenIndex16, depth16 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l16
					}
					position++
					goto l15
				l16:
					position, tokenIndex, depth = position16, tokenIndex16, depth16
				}
				depth--
				add(rulespaces, position14)
			}
			return true
		l13:
			position, tokenIndex, depth = position13, tokenIndex13, depth13
			return false
		},
		/* 3 table_def <- <(table_name sep+ table_lb sep+ columns sep+ table_rb)> */
		func() bool {
			position17, tokenIndex17, depth17 := position, tokenIndex, depth
			{
				position18 := position
				depth++
				if !_rules[ruletable_name]() {
					goto l17
				}
				if !_rules[rulesep]() {
					goto l17
				}
			l19:
				{
					position20, tokenIndex20, depth20 := position, tokenIndex, depth
					if !_rules[rulesep]() {
						goto l20
					}
					goto l19
				l20:
					position, tokenIndex, depth = position20, tokenIndex20, depth20
				}
				if !_rules[ruletable_lb]() {
					goto l17
				}
				if !_rules[rulesep]() {
					goto l17
				}
			l21:
				{
					position22, tokenIndex22, depth22 := position, tokenIndex, depth
					if !_rules[rulesep]() {
						goto l22
					}
					goto l21
				l22:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
				}
				if !_rules[rulecolumns]() {
					goto l17
				}
				if !_rules[rulesep]() {
					goto l17
				}
			l23:
				{
					position24, tokenIndex24, depth24 := position, tokenIndex, depth
					if !_rules[rulesep]() {
						goto l24
					}
					goto l23
				l24:
					position, tokenIndex, depth = position24, tokenIndex24, depth24
				}
				if !_rules[ruletable_rb]() {
					goto l17
				}
				depth--
				add(ruletable_def, position18)
			}
			return true
		l17:
			position, tokenIndex, depth = position17, tokenIndex17, depth17
			return false
		},
		/* 4 table_lb <- <'{'> */
		func() bool {
			position25, tokenIndex25, depth25 := position, tokenIndex, depth
			{
				position26 := position
				depth++
				if buffer[position] != rune('{') {
					goto l25
				}
				position++
				depth--
				add(ruletable_lb, position26)
			}
			return true
		l25:
			position, tokenIndex, depth = position25, tokenIndex25, depth25
			return false
		},
		/* 5 table_rb <- <('}' Action0)> */
		func() bool {
			position27, tokenIndex27, depth27 := position, tokenIndex, depth
			{
				position28 := position
				depth++
				if buffer[position] != rune('}') {
					goto l27
				}
				position++
				if !_rules[ruleAction0]() {
					goto l27
				}
				depth--
				add(ruletable_rb, position28)
			}
			return true
		l27:
			position, tokenIndex, depth = position27, tokenIndex27, depth27
			return false
		},
		/* 6 table_name <- <(<([a-z] / [A-Z] / '_')+> Action1)> */
		func() bool {
			position29, tokenIndex29, depth29 := position, tokenIndex, depth
			{
				position30 := position
				depth++
				{
					position31 := position
					depth++
					{
						position34, tokenIndex34, depth34 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l35
						}
						position++
						goto l34
					l35:
						position, tokenIndex, depth = position34, tokenIndex34, depth34
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l36
						}
						position++
						goto l34
					l36:
						position, tokenIndex, depth = position34, tokenIndex34, depth34
						if buffer[position] != rune('_') {
							goto l29
						}
						position++
					}
				l34:
				l32:
					{
						position33, tokenIndex33, depth33 := position, tokenIndex, depth
						{
							position37, tokenIndex37, depth37 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l38
							}
							position++
							goto l37
						l38:
							position, tokenIndex, depth = position37, tokenIndex37, depth37
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l39
							}
							position++
							goto l37
						l39:
							position, tokenIndex, depth = position37, tokenIndex37, depth37
							if buffer[position] != rune('_') {
								goto l33
							}
							position++
						}
					l37:
						goto l32
					l33:
						position, tokenIndex, depth = position33, tokenIndex33, depth33
					}
					depth--
					add(rulePegText, position31)
				}
				if !_rules[ruleAction1]() {
					goto l29
				}
				depth--
				add(ruletable_name, position30)
			}
			return true
		l29:
			position, tokenIndex, depth = position29, tokenIndex29, depth29
			return false
		},
		/* 7 columns <- <(column (sep* column)*)> */
		func() bool {
			position40, tokenIndex40, depth40 := position, tokenIndex, depth
			{
				position41 := position
				depth++
				if !_rules[rulecolumn]() {
					goto l40
				}
			l42:
				{
					position43, tokenIndex43, depth43 := position, tokenIndex, depth
				l44:
					{
						position45, tokenIndex45, depth45 := position, tokenIndex, depth
						if !_rules[rulesep]() {
							goto l45
						}
						goto l44
					l45:
						position, tokenIndex, depth = position45, tokenIndex45, depth45
					}
					if !_rules[rulecolumn]() {
						goto l43
					}
					goto l42
				l43:
					position, tokenIndex, depth = position43, tokenIndex43, depth43
				}
				depth--
				add(rulecolumns, position41)
			}
			return true
		l40:
			position, tokenIndex, depth = position40, tokenIndex40, depth40
			return false
		},
		/* 8 column <- <((column_name_with_relation / column_name_only) (spaces* ':' spaces* column_description)? Action2)> */
		func() bool {
			position46, tokenIndex46, depth46 := position, tokenIndex, depth
			{
				position47 := position
				depth++
				{
					position48, tokenIndex48, depth48 := position, tokenIndex, depth
					if !_rules[rulecolumn_name_with_relation]() {
						goto l49
					}
					goto l48
				l49:
					position, tokenIndex, depth = position48, tokenIndex48, depth48
					if !_rules[rulecolumn_name_only]() {
						goto l46
					}
				}
			l48:
				{
					position50, tokenIndex50, depth50 := position, tokenIndex, depth
				l52:
					{
						position53, tokenIndex53, depth53 := position, tokenIndex, depth
						if !_rules[rulespaces]() {
							goto l53
						}
						goto l52
					l53:
						position, tokenIndex, depth = position53, tokenIndex53, depth53
					}
					if buffer[position] != rune(':') {
						goto l50
					}
					position++
				l54:
					{
						position55, tokenIndex55, depth55 := position, tokenIndex, depth
						if !_rules[rulespaces]() {
							goto l55
						}
						goto l54
					l55:
						position, tokenIndex, depth = position55, tokenIndex55, depth55
					}
					if !_rules[rulecolumn_description]() {
						goto l50
					}
					goto l51
				l50:
					position, tokenIndex, depth = position50, tokenIndex50, depth50
				}
			l51:
				if !_rules[ruleAction2]() {
					goto l46
				}
				depth--
				add(rulecolumn, position47)
			}
			return true
		l46:
			position, tokenIndex, depth = position46, tokenIndex46, depth46
			return false
		},
		/* 9 column_description <- <(<(!'\n' .)+> Action3)> */
		func() bool {
			position56, tokenIndex56, depth56 := position, tokenIndex, depth
			{
				position57 := position
				depth++
				{
					position58 := position
					depth++
					{
						position61, tokenIndex61, depth61 := position, tokenIndex, depth
						if buffer[position] != rune('\n') {
							goto l61
						}
						position++
						goto l56
					l61:
						position, tokenIndex, depth = position61, tokenIndex61, depth61
					}
					if !matchDot() {
						goto l56
					}
				l59:
					{
						position60, tokenIndex60, depth60 := position, tokenIndex, depth
						{
							position62, tokenIndex62, depth62 := position, tokenIndex, depth
							if buffer[position] != rune('\n') {
								goto l62
							}
							position++
							goto l60
						l62:
							position, tokenIndex, depth = position62, tokenIndex62, depth62
						}
						if !matchDot() {
							goto l60
						}
						goto l59
					l60:
						position, tokenIndex, depth = position60, tokenIndex60, depth60
					}
					depth--
					add(rulePegText, position58)
				}
				if !_rules[ruleAction3]() {
					goto l56
				}
				depth--
				add(rulecolumn_description, position57)
			}
			return true
		l56:
			position, tokenIndex, depth = position56, tokenIndex56, depth56
			return false
		},
		/* 10 dot <- <'.'> */
		func() bool {
			position63, tokenIndex63, depth63 := position, tokenIndex, depth
			{
				position64 := position
				depth++
				if buffer[position] != rune('.') {
					goto l63
				}
				position++
				depth--
				add(ruledot, position64)
			}
			return true
		l63:
			position, tokenIndex, depth = position63, tokenIndex63, depth63
			return false
		},
		/* 11 column_name_with_relation <- <(column_name sep rarrow sep target_table_name dot target_column_name)> */
		func() bool {
			position65, tokenIndex65, depth65 := position, tokenIndex, depth
			{
				position66 := position
				depth++
				if !_rules[rulecolumn_name]() {
					goto l65
				}
				if !_rules[rulesep]() {
					goto l65
				}
				if !_rules[rulerarrow]() {
					goto l65
				}
				if !_rules[rulesep]() {
					goto l65
				}
				if !_rules[ruletarget_table_name]() {
					goto l65
				}
				if !_rules[ruledot]() {
					goto l65
				}
				if !_rules[ruletarget_column_name]() {
					goto l65
				}
				depth--
				add(rulecolumn_name_with_relation, position66)
			}
			return true
		l65:
			position, tokenIndex, depth = position65, tokenIndex65, depth65
			return false
		},
		/* 12 column_name_only <- <column_name> */
		func() bool {
			position67, tokenIndex67, depth67 := position, tokenIndex, depth
			{
				position68 := position
				depth++
				if !_rules[rulecolumn_name]() {
					goto l67
				}
				depth--
				add(rulecolumn_name_only, position68)
			}
			return true
		l67:
			position, tokenIndex, depth = position67, tokenIndex67, depth67
			return false
		},
		/* 13 column_name <- <(<([a-z] / [A-Z] / '_')+> Action4)> */
		func() bool {
			position69, tokenIndex69, depth69 := position, tokenIndex, depth
			{
				position70 := position
				depth++
				{
					position71 := position
					depth++
					{
						position74, tokenIndex74, depth74 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l75
						}
						position++
						goto l74
					l75:
						position, tokenIndex, depth = position74, tokenIndex74, depth74
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l76
						}
						position++
						goto l74
					l76:
						position, tokenIndex, depth = position74, tokenIndex74, depth74
						if buffer[position] != rune('_') {
							goto l69
						}
						position++
					}
				l74:
				l72:
					{
						position73, tokenIndex73, depth73 := position, tokenIndex, depth
						{
							position77, tokenIndex77, depth77 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l78
							}
							position++
							goto l77
						l78:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l79
							}
							position++
							goto l77
						l79:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
							if buffer[position] != rune('_') {
								goto l73
							}
							position++
						}
					l77:
						goto l72
					l73:
						position, tokenIndex, depth = position73, tokenIndex73, depth73
					}
					depth--
					add(rulePegText, position71)
				}
				if !_rules[ruleAction4]() {
					goto l69
				}
				depth--
				add(rulecolumn_name, position70)
			}
			return true
		l69:
			position, tokenIndex, depth = position69, tokenIndex69, depth69
			return false
		},
		/* 14 rarrow <- <(rdotarrow / rlinearrow)> */
		func() bool {
			position80, tokenIndex80, depth80 := position, tokenIndex, depth
			{
				position81 := position
				depth++
				{
					position82, tokenIndex82, depth82 := position, tokenIndex, depth
					if !_rules[rulerdotarrow]() {
						goto l83
					}
					goto l82
				l83:
					position, tokenIndex, depth = position82, tokenIndex82, depth82
					if !_rules[rulerlinearrow]() {
						goto l80
					}
				}
			l82:
				depth--
				add(rulerarrow, position81)
			}
			return true
		l80:
			position, tokenIndex, depth = position80, tokenIndex80, depth80
			return false
		},
		/* 15 rdotarrow <- <('.' '.' '>' Action5)> */
		func() bool {
			position84, tokenIndex84, depth84 := position, tokenIndex, depth
			{
				position85 := position
				depth++
				if buffer[position] != rune('.') {
					goto l84
				}
				position++
				if buffer[position] != rune('.') {
					goto l84
				}
				position++
				if buffer[position] != rune('>') {
					goto l84
				}
				position++
				if !_rules[ruleAction5]() {
					goto l84
				}
				depth--
				add(rulerdotarrow, position85)
			}
			return true
		l84:
			position, tokenIndex, depth = position84, tokenIndex84, depth84
			return false
		},
		/* 16 rlinearrow <- <('-' '>' Action6)> */
		func() bool {
			position86, tokenIndex86, depth86 := position, tokenIndex, depth
			{
				position87 := position
				depth++
				if buffer[position] != rune('-') {
					goto l86
				}
				position++
				if buffer[position] != rune('>') {
					goto l86
				}
				position++
				if !_rules[ruleAction6]() {
					goto l86
				}
				depth--
				add(rulerlinearrow, position87)
			}
			return true
		l86:
			position, tokenIndex, depth = position86, tokenIndex86, depth86
			return false
		},
		/* 17 target_table_name <- <(<([a-z] / [A-Z] / '_')+> Action7)> */
		func() bool {
			position88, tokenIndex88, depth88 := position, tokenIndex, depth
			{
				position89 := position
				depth++
				{
					position90 := position
					depth++
					{
						position93, tokenIndex93, depth93 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l94
						}
						position++
						goto l93
					l94:
						position, tokenIndex, depth = position93, tokenIndex93, depth93
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l95
						}
						position++
						goto l93
					l95:
						position, tokenIndex, depth = position93, tokenIndex93, depth93
						if buffer[position] != rune('_') {
							goto l88
						}
						position++
					}
				l93:
				l91:
					{
						position92, tokenIndex92, depth92 := position, tokenIndex, depth
						{
							position96, tokenIndex96, depth96 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l97
							}
							position++
							goto l96
						l97:
							position, tokenIndex, depth = position96, tokenIndex96, depth96
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l98
							}
							position++
							goto l96
						l98:
							position, tokenIndex, depth = position96, tokenIndex96, depth96
							if buffer[position] != rune('_') {
								goto l92
							}
							position++
						}
					l96:
						goto l91
					l92:
						position, tokenIndex, depth = position92, tokenIndex92, depth92
					}
					depth--
					add(rulePegText, position90)
				}
				if !_rules[ruleAction7]() {
					goto l88
				}
				depth--
				add(ruletarget_table_name, position89)
			}
			return true
		l88:
			position, tokenIndex, depth = position88, tokenIndex88, depth88
			return false
		},
		/* 18 target_column_name <- <(<([a-z] / [A-Z] / '_')+> Action8)> */
		func() bool {
			position99, tokenIndex99, depth99 := position, tokenIndex, depth
			{
				position100 := position
				depth++
				{
					position101 := position
					depth++
					{
						position104, tokenIndex104, depth104 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l105
						}
						position++
						goto l104
					l105:
						position, tokenIndex, depth = position104, tokenIndex104, depth104
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l106
						}
						position++
						goto l104
					l106:
						position, tokenIndex, depth = position104, tokenIndex104, depth104
						if buffer[position] != rune('_') {
							goto l99
						}
						position++
					}
				l104:
				l102:
					{
						position103, tokenIndex103, depth103 := position, tokenIndex, depth
						{
							position107, tokenIndex107, depth107 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l108
							}
							position++
							goto l107
						l108:
							position, tokenIndex, depth = position107, tokenIndex107, depth107
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l109
							}
							position++
							goto l107
						l109:
							position, tokenIndex, depth = position107, tokenIndex107, depth107
							if buffer[position] != rune('_') {
								goto l103
							}
							position++
						}
					l107:
						goto l102
					l103:
						position, tokenIndex, depth = position103, tokenIndex103, depth103
					}
					depth--
					add(rulePegText, position101)
				}
				if !_rules[ruleAction8]() {
					goto l99
				}
				depth--
				add(ruletarget_column_name, position100)
			}
			return true
		l99:
			position, tokenIndex, depth = position99, tokenIndex99, depth99
			return false
		},
		/* 19 EOT <- <!.> */
		func() bool {
			position110, tokenIndex110, depth110 := position, tokenIndex, depth
			{
				position111 := position
				depth++
				{
					position112, tokenIndex112, depth112 := position, tokenIndex, depth
					if !matchDot() {
						goto l112
					}
					goto l110
				l112:
					position, tokenIndex, depth = position112, tokenIndex112, depth112
				}
				depth--
				add(ruleEOT, position111)
			}
			return true
		l110:
			position, tokenIndex, depth = position110, tokenIndex110, depth110
			return false
		},
		/* 21 Action0 <- <{
		    p.Tables = append(p.Tables, *p.table)
		}> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		nil,
		/* 23 Action1 <- <{
		    p.table = &Table{
		    Name: text,
			       Columns: make([]Column, 0),
			   }
		}> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 24 Action2 <- <{
		    p.table.Columns = append(p.table.Columns, *p.column)
		}> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 25 Action3 <- <{
		    p.column.Description = text
		}> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 26 Action4 <- <{
			p.column = &Column{
			  Name: text,
			}
		}> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 27 Action5 <- <{
		    p.column.Relation = &Relation{
		        LineType: DotLine,
		    }
		}> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 28 Action6 <- <{
		    p.column.Relation = &Relation{
		        LineType: NormalLine,
		    }
		}> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 29 Action7 <- <{
		    p.column.Relation.TableName = text
		}> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 30 Action8 <- <{
		    p.column.Relation.ColumnName = text
		}> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
	}
	p.rules = _rules
}
