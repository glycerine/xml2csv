package main

// Copyright (C) 2023, Jason E. Aten, Ph.D.
// License: MIT; see LICENSE file.

// xml2csv: parse an XML file on stdin, and write out a csv file version of it to stdout.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ls -1 *.xml > list
// for i in `cat list`; do k=${i/xml}; ./xml2csv < $i > ${k}csv; done

// escape double quotes
func esc(s string) (r string) {
	return strings.ReplaceAll(s, `"`, `""`)
}

var newline = []byte("\n")

// tag represents one XML tag, like "<person>" in the line "<person>John Smith</person>".
// The "</person>" is also a tag, a closing tag, and we may point to our paired begin or end tag.
type tag struct {
	btwn    string // what is between the < > angle brackets
	name    string // the name of the node, stopping after the first whitespace. <name or </name
	beg     int    // byte position in the file
	endx    int
	isClose bool // do we start with </name

	isSimple   bool
	selfClosed bool
	endTag     *tag
	begTag     *tag
	content    string

	firstChild *tag
	nextSib    *tag
	numChild   int

	discard  bool // mark true if this is a simple tag with no content variation in content
	compound bool // if numChild > 0
	colname  string
	dupcount int // number of times this colname is duplicated among siblings
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (t *tag) String() string {
	//return fmt.Sprintf("%v%v", t.btwn, t.content)
	return fmt.Sprintf("%v:%v", strings.ReplaceAll(t.name, ":", "_"), t.content)
}

// split into tags
func tokenize(by []byte) (tags []*tag) {
	n := len(by)
	var i, j, k, beg, endx int
	for k = bytes.IndexByte(by[i:], '<'); k >= 0 && i < n; k = bytes.IndexByte(by[i:], '<') {
		beg = i + k
		j = bytes.IndexByte(by[i:], '>')
		if j == -1 {
			panic(fmt.Sprintf("begin tag without matching end tag!: beg=%v; by[beg:(beg+100)]='%v'", beg, string(by[beg:intMin(n, beg+100)])))
		}
		endx = i + j + 1
		mytag := &tag{
			beg:  beg,
			endx: endx,
		}
		mytag.btwn = string(by[mytag.beg:mytag.endx])
		//vv("n=%v; i=%v; beg=%v, endx=%v between='%v'", n, i, beg, mytag.endx, mytag.btwn)
		if mytag.btwn[1] == '/' {
			mytag.isClose = true
			mytag.name = mytag.btwn[2 : len(mytag.btwn)-1]
		} else {
			mytag.name = mytag.btwn[1 : len(mytag.btwn)-1]
		}
		// get actual name, by reducing
		// "namespace:tag rdf:about=..." -> "namespace:tag"
		space := strings.Index(mytag.name, " ")
		if space >= 0 {
			mytag.name = mytag.name[:space]
		}
		mytag.name = strings.TrimSpace(mytag.name)

		// set initial colname here, without the namespace "institute:" or "schema:" prefix
		mytag.colname = stripNamespace(mytag.name)

		// handle self-closing <tag />, like
		// <schema:url rdf:resource="http://www..."/>
		if mytag.btwn[len(mytag.btwn)-2] == '/' {
			mytag.selfClosed = true
		}

		tags = append(tags, mytag)
		i = endx
	}
	return
}

func stripNamespace(s string) (r string) {
	if !strings.Contains(s, ":") {
		return s
	}
	r = strings.SplitN(s, ":", 2)[1]
	return
}

type Map struct {
	m       map[string]bool
	discard bool
}

func newMap() *Map {
	return &Map{m: make(map[string]bool)}
}

func main() {

	// convert XML file to tree of tag(s)

	data, err := io.ReadAll(os.Stdin)
	panicOn(err)

	tags := tokenize(data)
	n := len(tags)

	// build up the parse tree here, by filling in
	// 	firstChild, nextSib

	var tree *tag
	var stack []*tag

	push := func(t *tag) {
		stack = append(stack, t)
	}
	pop := func() {
		m := len(stack)
		if m > 0 {
			stack = stack[:m-1]
		}
	}
	top := func() *tag {
		m := len(stack)
		if m == 0 {
			return nil
		}
		return stack[m-1]
	}
	add_child := func(t *tag) {
		m := len(stack)
		if m == 0 {
			panic("cannot add child to empty stack")
		}
		tp := top()
		if tp.firstChild == nil {
			tp.firstChild = t
			tp.numChild = 1
		} else {
			tp.numChild++
			sib := tp.firstChild
			for sib.nextSib != nil {
				sib = sib.nextSib
			}
			sib.nextSib = t
		}
	}

	// keep simple stats so we can discard no-content columns.
	// NB: did not get this simpleMap mechanism fully working as of yet; seemed to be
	// throwing out baby with the bathwater.
	//
	simpleMap := make(map[string]*Map)
	addSimple := func(t *tag) {
		m, ok := simpleMap[t.name]
		if !ok {
			m = newMap()
			simpleMap[t.name] = m
		}
		m.m[strings.TrimSpace(t.content)] = true
	}

	for i := 0; i < len(tags); i++ {
		tag := tags[i]
		if strings.HasPrefix(tag.btwn, "<?xml") {
			// ignore <?xml version="1.0"?>
			continue
		}

		if tree == nil {
			tree = tag
			push(tag)
			continue
		}

		if tag.selfClosed {
			add_child(tag)
			continue
		}

		// peek 1 ahead for simple tags
		if i < n-1 {
			if !tag.isClose && tags[i+1].isClose && tag.name == tags[i+1].name {

				tag.isSimple = true
				endTag := tags[i+1]
				endTag.isSimple = true
				tag.endTag = endTag
				endTag.begTag = tag
				tag.content = string(data[tag.endx:endTag.beg])
				addSimple(tag)

				// skip past the closing tag
				i++
				add_child(tag)
				continue
			}

			// have a compound tag
			if !tag.isClose {
				if len(stack) > 0 {
					add_child(tag)
				}
				push(tag)
			} else {
				if tag.name != top().name {
					panic(fmt.Sprintf("maybe bad xml at i=%v trying to close '%v' against open '%v'", i, tag.name, top().name))
				}
				pop()
			}
		}
		//vv("tag = '%v'", tag)
	}

	exclude := noteDiscards(simpleMap)
	//vv("exclude dicards = '%v'", exclude)
	markZeroContentTags(tags, simpleMap)

	//vv("top node has %v children", tree.numChild)

	//printXMLTree(tree, 0)

	stack = stack[:0] // clear the stack

	var colnm []string
	colmap := make(map[string]int)
	cur := tree.firstChild
	sibnames := make(map[string]int)
	genColnames(&colnm, colmap, stack, sibnames, cur)

	// sort the columns for final output
	var final []string
	for _, cn := range colnm {
		// excludes does nothing at the moment because it includes the namespace for
		// dis-ambiguation, whereas the colnm has had the namespace stripped out.
		if !exclude[cn] {
			final = append(final, cn)
		}
	}
	sort.Strings(final)
	fmap := make(map[string]int)
	for i, s := range final {
		fmap[s] = i
	}

	// why no _id field? b/c was wrongly being detected as a discard, weird.
	//vv("colnm (%v) = '%#v'", len(colnm), colnm)
	//vv("final (%v) = '%#v'", len(final), final)

	// print header
	for i, s := range final {
		if i == 0 {
			fmt.Printf("%v", s)
		} else {
			fmt.Printf(",%v", s)
		}
	}
	fmt.Printf("\n")
	printAsCsv(tree, fmap)
}

func printAsCsv(tree *tag, fmap map[string]int) {

	cur := tree.firstChild
	for cur != nil {
		fmt.Printf("%v\n", asCsvLine(cur.firstChild, fmap))
		cur = cur.nextSib
	}
}

func asCsvLine(cur *tag, fmap map[string]int) string {
	fld := make([]string, len(fmap))

	fillFields(cur, fmap, fld)

	return strings.Join(fld, ",")
}

func fillFields(cur *tag, fmap map[string]int, fld []string) {
	if cur == nil {
		return
	}
	w, ok := fmap[cur.colname]
	if ok {
		fld[w] = `"` + esc(trimAllSpace(cur.content)) + `"`
	}

	if cur.firstChild != nil {
		fillFields(cur.firstChild, fmap, fld)
	}
	if cur.nextSib != nil {
		fillFields(cur.nextSib, fmap, fld)
	}
}

func prefix(stack []*tag) (r string) {
	for i, tag := range stack {
		if i == 0 {
			// skip the top level record name
			continue
		}
		//r += strings.ReplaceAll(tag.colname, ":", "_") + "_"
		r += tag.colname
		r += "_"
	}
	return
}

// Use sibnames to detect repeated xml elements that have the same tag.
//
func genColnames(colnm *[]string, colmap map[string]int, stack []*tag, sibnames map[string]int, cur *tag) {

	if cur == nil {
		return
	}
	if cur.name == "schema:created" || cur.name == "schema:modified" { // || cur.discard {
		// can skip these but have to do their siblings, and since
		// we use nextSib links, these records are the only way to
		// get to their siblings, so it must be done now.
		if cur.nextSib != nil {
			genColnames(colnm, colmap, stack, sibnames, cur.nextSib)
		}
		return
	}

	//vv("gen called on '%v'", cur)

	// detect any duplicated names between siblings. uniquify with an increasing digit suffix.
	// have to do this before we push our cur onto the stack.
	// also we don't want to give each depth 1 record its own name, so require len(stack) > 0
	if len(stack) > 0 {
		dup, already := sibnames[cur.name]
		if already {
			sibnames[cur.name] = dup + 1
			cur.dupcount = dup + 1
			cur.colname = fmt.Sprintf("%v%v", stripNamespace(cur.name), cur.dupcount)
			//vv("detected duplicate cur.name='%v'; cur.dupcount=%v -> cur.colname='%v'; sibnames is now: '%v'; stack[0]='%v'", cur.name, cur.dupcount, cur.colname, sibnames, stack[0].btwn)
		} else {
			sibnames[cur.name] = 0
		}
	}

	if cur.numChild == 0 {
		nm := prefix(stack) + cur.colname
		//vv("at leaf, nm = '%v' from cur.colname='%v'; cur.name='%v'", nm, cur.colname, cur.name)

		//if cur.colname == "ID" {
		//vv("found simple ID tag: '%v' with colname = '%v'; nm='%v'", cur, cur.colname, nm)
		//}

		k, ok := colmap[nm]
		if !ok {
			colmap[nm] = len(*colnm)
			*colnm = append(*colnm, nm)
			cur.colname = nm
		} else {
			cur.colname = (*colnm)[k]
		}
	} else {
		//vv("cur '%v' has %v children", cur.name, cur.numChild)
		cur.compound = true
		if cur.firstChild != nil {
			genColnames(colnm, colmap, append(stack, cur), make(map[string]int), cur.firstChild)
		}
	}

	if cur.nextSib != nil {
		genColnames(colnm, colmap, stack, sibnames, cur.nextSib)
	}
}

// if a column has only empty string "" or the just the value "None", or just both of these,
// the mark it as a discard.
func noteDiscards(simpleMap map[string]*Map) (r map[string]bool) {
	r = make(map[string]bool)
	for name, m := range simpleMap {
		_ = name
		n := len(m.m)
		if n == 0 {
			m.discard = true
			//vv("discarding tag '%v'", name)
			r[name] = true
			continue
		}
		if n > 2 {
			continue
		}
		keep := false
		for s := range m.m {
			if s != "" && s != "None" {
				keep = true
			}
		}
		if !keep {
			m.discard = true
			//vv("discarding tag '%v'", name)
			r[name] = true
		}
	}
	return
}

func markZeroContentTags(tags []*tag, simpleMap map[string]*Map) {
	for _, tag := range tags {
		m, ok := simpleMap[tag.name]
		if !ok {
			continue
		}
		tag.discard = m.discard
	}
}

// printXMLTree re-displays the parsed XML
func printXMLTree(cur *tag, level int) {

	indent := strings.Repeat(" ", level*4)
	fmt.Printf("%v%v\n", indent, cur)
	if cur.firstChild != nil {
		printXMLTree(cur.firstChild, level+1)
	}
	// all children have been printed
	if cur.nextSib != nil {
		printXMLTree(cur.nextSib, level)
	}
}

func trimAllSpace(s string) (r string) {
	r = strings.TrimSpace(s)
	if len(r) == 0 {
		return
	}
	return s
}
