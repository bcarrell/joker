package main

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"unicode"
	"unicode/utf8"
)

type (
	Equality interface {
		Equals(interface{}) bool
	}
	Object interface {
		Equality
		ToString(escape bool) string
	}
	Char       rune
	Double     float64
	Int        int
	BigInt     big.Int
	BigFloat   big.Float
	Ratio      big.Rat
	Bool       bool
	Nil        struct{}
	Keyword    string
	Symbol     string
	String     string
	Regex      string
	ReadObject struct {
		line   int
		column int
		obj    Object
	}
	ReadError struct {
		line   int
		column int
		msg    string
	}
	ReadFunc func(reader *Reader) ReadObject
)

const EOF = -1

var (
	ARGS   map[int]Symbol
	GENSYM int
)

func readStub(reader *Reader) ReadObject {
	return Read(reader)
}

var DATA_READERS = map[Symbol]ReadFunc{}
var NIL Nil

func init() {
	DATA_READERS[Symbol("inst")] = readStub
	DATA_READERS[Symbol("uuid")] = readStub
}

func (ro ReadObject) Equals(other interface{}) bool {
	switch other := other.(type) {
	case *ReadObject:
		return ro.obj.Equals(other.obj)
	case Object:
		return ro.obj.Equals(other)
	default:
		return false
	}
}

func (ro ReadObject) ToString(escape bool) string {
	return ro.obj.ToString(escape)
}

func (n Nil) ToString(escape bool) string {
	return "nil"
}

func (n Nil) Equals(other interface{}) bool {
	return n == other
}

func (rat *Ratio) ToString(escape bool) string {
	return (*big.Rat)(rat).String()
}

func (rat *Ratio) Equals(other interface{}) bool {
	if rat == other {
		return true
	}
	switch r := other.(type) {
	case *Ratio:
		return ((*big.Rat)(rat)).Cmp((*big.Rat)(r)) == 0
	case *BigInt:
		var otherRat big.Rat
		otherRat.SetInt((*big.Int)(r))
		return ((*big.Rat)(rat)).Cmp(&otherRat) == 0
	case Int:
		var otherRat big.Rat
		otherRat.SetInt64(int64(r))
		return ((*big.Rat)(rat)).Cmp(&otherRat) == 0
	}
	return false
}

func (bi *BigInt) ToString(escape bool) string {
	return (*big.Int)(bi).String() + "N"
}

func (bi *BigInt) Equals(other interface{}) bool {
	if bi == other {
		return true
	}
	switch b := other.(type) {
	case *BigInt:
		return ((*big.Int)(bi)).Cmp((*big.Int)(b)) == 0
	case Int:
		bi2 := big.NewInt(int64(b))
		return ((*big.Int)(bi)).Cmp(bi2) == 0
	}
	return false
}

func (bf *BigFloat) ToString(escape bool) string {
	return (*big.Float)(bf).Text('g', 256) + "M"
}

func (bf *BigFloat) Equals(other interface{}) bool {
	if bf == other {
		return true
	}
	switch b := other.(type) {
	case *BigFloat:
		return ((*big.Float)(bf)).Cmp((*big.Float)(b)) == 0
	case Double:
		bf2 := big.NewFloat(float64(b))
		return ((*big.Float)(bf)).Cmp(bf2) == 0
	}
	return false
}

func (c Char) ToString(escape bool) string {
	if escape {
		return escapeRune(rune(c))
	}
	return string(c)
}

func (c Char) Equals(other interface{}) bool {
	return c == other
}

func (d Double) ToString(escape bool) string {
	return fmt.Sprintf("%f", float64(d))
}

func (d Double) Equals(other interface{}) bool {
	return d == other
}

func (i Int) ToString(escape bool) string {
	return fmt.Sprintf("%d", int(i))
}

func (i Int) Equals(other interface{}) bool {
	return i == other
}

func (b Bool) ToString(escape bool) string {
	return fmt.Sprintf("%t", bool(b))
}

func (b Bool) Equals(other interface{}) bool {
	return b == other
}

func (k Keyword) ToString(escape bool) string {
	return string(k)
}

func (k Keyword) Equals(other interface{}) bool {
	return k == other
}

func (rx Regex) ToString(escape bool) string {
	if escape {
		return "#" + escapeString(string(rx))
	}
	return "#" + string(rx)
}

func (rx Regex) Equals(other interface{}) bool {
	return rx == other
}

func (s Symbol) ToString(escape bool) string {
	return string(s)
}

func (s Symbol) Equals(other interface{}) bool {
	return s == other
}

func (s String) ToString(escape bool) string {
	if escape {
		return escapeString(string(s))
	}
	return string(s)
}

func (s String) Equals(other interface{}) bool {
	return s == other
}

func escapeRune(r rune) string {
	switch r {
	case ' ':
		return "\\space"
	case '\n':
		return "\\newline"
	case '\t':
		return "\\tab"
	case '\r':
		return "\\return"
	case '\b':
		return "\\backspace"
	case '\f':
		return "\\formfeed"
	default:
		return "\\" + string(r)
	}
}

func escapeString(str string) string {
	var b bytes.Buffer
	b.WriteRune('"')
	for _, r := range str {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\t':
			b.WriteString("\\t")
		case '\r':
			b.WriteString("\\r")
		case '\n':
			b.WriteString("\\n")
		case '\f':
			b.WriteString("\\f")
		case '\b':
			b.WriteString("\\b")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteRune('"')
	return b.String()
}

func MakeReadError(reader *Reader, msg string) ReadError {
	return ReadError{
		line:   reader.line,
		column: reader.column,
		msg:    msg,
	}
}

func MakeReadObject(reader *Reader, obj Object) ReadObject {
	return ReadObject{
		line:   reader.line,
		column: reader.column,
		obj:    obj,
	}
}

func DeriveReadObject(base ReadObject, obj Object) ReadObject {
	return ReadObject{
		line:   base.line,
		column: base.column,
		obj:    obj,
	}
}

func (err ReadError) Error() string {
	return fmt.Sprintf("stdin:%d:%d: %s", err.line, err.column, err.msg)
}

func isDelimiter(r rune) bool {
	switch r {
	case '(', ')', '[', ']', '{', '}', '"', ';', EOF, ',', '\\':
		return true
	}
	return unicode.IsSpace(r)
}

func eatString(reader *Reader, str string) {
	for _, sr := range str {
		if r := reader.Get(); r != sr {
			panic(MakeReadError(reader, fmt.Sprintf("Unexpected character %U", r)))
		}
	}
}

func peekExpectedDelimiter(reader *Reader) {
	r := reader.Peek()
	if !isDelimiter(r) {
		panic(MakeReadError(reader, "Character not followed by delimiter"))
	}
}

func readSpecialCharacter(reader *Reader, ending string, r rune) ReadObject {
	eatString(reader, ending)
	peekExpectedDelimiter(reader)
	return MakeReadObject(reader, Char(r))
}

func eatWhitespace(reader *Reader) {
	r := reader.Get()
	for r != EOF {
		if unicode.IsSpace(r) || r == ',' {
			r = reader.Get()
			continue
		}
		if r == ';' || (r == '#' && reader.Peek() == '!') {
			for r != '\n' && r != EOF {
				r = reader.Get()
			}
			r = reader.Get()
			continue
		}
		if r == '#' && reader.Peek() == '_' {
			reader.Get()
			Read(reader)
			r = reader.Get()
			continue
		}
		reader.Unget()
		break
	}
}

func readUnicodeCharacter(reader *Reader, length, base int) ReadObject {
	var b bytes.Buffer
	for n := reader.Get(); !isDelimiter(n); n = reader.Get() {
		b.WriteRune(n)
	}
	reader.Unget()
	str := b.String()
	if len(str) != length {
		panic(MakeReadError(reader, "Invalid unicode character: \\o"+str))
	}
	i, err := strconv.ParseInt(str, base, 32)
	if err != nil {
		panic(MakeReadError(reader, "Invalid unicode character: \\o"+str))
	}
	peekExpectedDelimiter(reader)
	return MakeReadObject(reader, Char(rune(i)))
}

func readCharacter(reader *Reader) ReadObject {
	r := reader.Get()
	if r == EOF {
		panic(MakeReadError(reader, "Incomplete character literal"))
	}
	switch r {
	case 's':
		if reader.Peek() == 'p' {
			return readSpecialCharacter(reader, "pace", ' ')
		}
	case 'n':
		if reader.Peek() == 'e' {
			return readSpecialCharacter(reader, "ewline", '\n')
		}
	case 't':
		if reader.Peek() == 'a' {
			return readSpecialCharacter(reader, "ab", '\t')
		}
	case 'f':
		if reader.Peek() == 'o' {
			return readSpecialCharacter(reader, "ormfeed", '\f')
		}
	case 'b':
		if reader.Peek() == 'a' {
			return readSpecialCharacter(reader, "ackspace", '\b')
		}
	case 'r':
		if reader.Peek() == 'e' {
			return readSpecialCharacter(reader, "eturn", '\r')
		}
	case 'u':
		if !isDelimiter(reader.Peek()) {
			return readUnicodeCharacter(reader, 4, 16)
		}
	case 'o':
		if !isDelimiter(reader.Peek()) {
			readUnicodeCharacter(reader, 3, 8)
		}
	}
	peekExpectedDelimiter(reader)
	return MakeReadObject(reader, Char(r))
}

func scanBigInt(str string, base int, err error, reader *Reader) ReadObject {
	var bi big.Int
	if _, ok := bi.SetString(str, base); !ok {
		panic(err)
	}
	res := BigInt(bi)
	return MakeReadObject(reader, &res)
}

func scanRatio(str string, err error, reader *Reader) ReadObject {
	var rat big.Rat
	if _, ok := rat.SetString(str); !ok {
		panic(err)
	}
	res := Ratio(rat)
	return MakeReadObject(reader, &res)
}

func scanBigFloat(str string, err error, reader *Reader) ReadObject {
	var bf big.Float
	if _, ok := bf.SetPrec(256).SetString(str); !ok {
		panic(err)
	}
	res := BigFloat(bf)
	return MakeReadObject(reader, &res)
}

func scanInt(str string, base int, err error, reader *Reader) ReadObject {
	i, e := strconv.ParseInt(str, base, 0)
	if e != nil {
		return scanBigInt(str, base, err, reader)
	}
	return MakeReadObject(reader, Int(int(i)))
}

func readNumber(reader *Reader) ReadObject {
	var b bytes.Buffer
	isDouble, isHex, isExp, isRatio, base, nonDigits := false, false, false, false, "", 0
	d := reader.Get()
	last := d
	for !isDelimiter(d) {
		switch d {
		case '.':
			isDouble = true
		case '/':
			isRatio = true
		case 'x', 'X':
			isHex = true
		case 'e', 'E':
			isExp = true
		case 'r', 'R':
			if base == "" {
				base = b.String()
				b.Reset()
				last = d
				d = reader.Get()
				continue
			}
		}
		if !unicode.IsDigit(d) {
			nonDigits++
		}
		b.WriteRune(d)
		last = d
		d = reader.Get()
	}
	reader.Unget()
	str := b.String()
	if base != "" {
		invalidNumberError := MakeReadError(reader, fmt.Sprintf("Invalid number: %s", base+"r"+str))
		baseInt, err := strconv.ParseInt(base, 0, 0)
		if err != nil {
			panic(invalidNumberError)
		}
		if base[0] == '-' {
			baseInt = -baseInt
			str = "-" + str
		}
		if baseInt < 2 || baseInt > 36 {
			panic(invalidNumberError)
		}
		return scanInt(str, int(baseInt), invalidNumberError, reader)
	}
	invalidNumberError := MakeReadError(reader, fmt.Sprintf("Invalid number: %s", str))
	if isRatio {
		if nonDigits > 2 || nonDigits > 1 && str[0] != '-' {
			panic(invalidNumberError)
		}
		return scanRatio(str, invalidNumberError, reader)
	}
	if last == 'N' {
		b.Truncate(b.Len() - 1)
		return scanBigInt(b.String(), 0, invalidNumberError, reader)
	}
	if last == 'M' {
		b.Truncate(b.Len() - 1)
		return scanBigFloat(b.String(), invalidNumberError, reader)
	}
	if isDouble || (!isHex && isExp) {
		dbl, err := strconv.ParseFloat(str, 64)
		if err != nil {
			panic(invalidNumberError)
		}
		return MakeReadObject(reader, Double(dbl))
	}
	return scanInt(str, 0, invalidNumberError, reader)
}

func isSymbolInitial(r rune) bool {
	switch r {
	case '*', '+', '!', '-', '_', '?', ':', '=', '<', '>', '&', '.', '%', '$', '|':
		return true
	}
	return unicode.IsLetter(r)
}

func isSymbolRune(r rune) bool {
	return isSymbolInitial(r) || unicode.IsDigit(r) || r == '#' || r == '/' || r == '\''
}

func readSymbol(reader *Reader, first rune) ReadObject {
	var b bytes.Buffer
	b.WriteRune(first)
	var lastAdded rune
	r := reader.Get()
	for isSymbolRune(r) {
		if r == ':' {
			if b.Len() > 1 && lastAdded == ':' {
				panic(MakeReadError(reader, "Invalid use of ':' in symbol name"))
			}
		}
		b.WriteRune(r)
		lastAdded = r
		r = reader.Get()
	}
	if lastAdded == ':' || lastAdded == '/' {
		panic(MakeReadError(reader, fmt.Sprintf("Invalid use of %c in symbol name", lastAdded)))
	}
	reader.Unget()
	str := b.String()
	switch {
	case str == "nil":
		return MakeReadObject(reader, NIL)
	case str == "true":
		return MakeReadObject(reader, Bool(true))
	case str == "false":
		return MakeReadObject(reader, Bool(false))
	case first == ':':
		return MakeReadObject(reader, Keyword(str))
	default:
		return MakeReadObject(reader, Symbol(str))
	}
}

func readString(reader *Reader, isRegex bool) ReadObject {
	var b bytes.Buffer
	r := reader.Get()
	for r != '"' {
		if r == '\\' {
			r = reader.Get()
			switch r {
			case 'n':
				r = '\n'
			case 't':
				r = '\t'
			case 'r':
				r = '\r'
			case 'b':
				r = '\b'
			case 'f':
				r = '\f'
			case 'u':
				var b bytes.Buffer
				n := reader.Get()
				for i := 0; i < 4 && n != '"'; i++ {
					b.WriteRune(n)
					n = reader.Get()
				}
				reader.Unget()
				str := b.String()
				if len(str) != 4 {
					panic(MakeReadError(reader, "Invalid unicode escape: \\u"+str))
				}
				i, err := strconv.ParseInt(str, 16, 32)
				if err != nil {
					panic(MakeReadError(reader, "Invalid unicode escape: \\u"+str))
				}
				r = rune(i)
			}
		}
		if r == EOF {
			panic(MakeReadError(reader, "Non-terminated string literal"))
		}
		b.WriteRune(r)
		r = reader.Get()
	}
	if isRegex {
		return MakeReadObject(reader, Regex(b.String()))
	}
	return MakeReadObject(reader, String(b.String()))
}

func readList(reader *Reader) ReadObject {
	s := make([]ReadObject, 0, 10)
	eatWhitespace(reader)
	r := reader.Peek()
	for r != ')' {
		obj := Read(reader)
		s = append(s, obj)
		eatWhitespace(reader)
		r = reader.Peek()
	}
	reader.Get()
	list := EmptyList
	for i := len(s) - 1; i >= 0; i-- {
		list = list.Conj(s[i])
	}
	return MakeReadObject(reader, list)
}

func readVector(reader *Reader) ReadObject {
	result := EmptyVector
	eatWhitespace(reader)
	r := reader.Peek()
	for r != ']' {
		obj := Read(reader)
		result = result.conj(obj)
		eatWhitespace(reader)
		r = reader.Peek()
	}
	reader.Get()
	return MakeReadObject(reader, result)
}

func readMap(reader *Reader) ReadObject {
	m := EmptyArrayMap()
	eatWhitespace(reader)
	r := reader.Peek()
	for r != '}' {
		key := Read(reader)
		value := Read(reader)
		if !m.Add(key, value) {
			panic(MakeReadError(reader, "Duplicate key "+key.ToString(false)))
		}
		eatWhitespace(reader)
		r = reader.Peek()
	}
	reader.Get()
	return MakeReadObject(reader, m)
}

func readSet(reader *Reader) ReadObject {
	set := EmptySet()
	eatWhitespace(reader)
	r := reader.Peek()
	for r != '}' {
		obj := Read(reader)
		if !set.Add(obj) {
			panic(MakeReadError(reader, "Duplicate key "+obj.ToString(false)))
		}
		eatWhitespace(reader)
		r = reader.Peek()
	}
	reader.Get()
	return MakeReadObject(reader, set)
}

func makeQuote(obj ReadObject, quote Symbol) ReadObject {
	return ReadObject{column: obj.column, line: obj.line, obj: EmptyList.Cons(obj).Cons(quote)}
}

func readMeta(reader *Reader) ReadObject {
	obj := Read(reader)
	switch obj.obj.(type) {
	case *ArrayMap:
		return obj
	case String, Symbol:
		return DeriveReadObject(obj, &ArrayMap{arr: []Object{DeriveReadObject(obj, Keyword(":tag")), obj}})
	case Keyword:
		return DeriveReadObject(obj, &ArrayMap{arr: []Object{obj, DeriveReadObject(obj, Bool(true))}})
	default:
		panic(MakeReadError(reader, "Metadata must be Symbol, Keyword, String or Map"))
	}
}

func makeWithMeta(obj ReadObject, meta ReadObject) ReadObject {
	return DeriveReadObject(obj, NewListFrom(DeriveReadObject(meta, Symbol("with-meta")), meta, obj))
}

func fillInMissingArgs(args map[int]Symbol) {
	max := 0
	for k := range args {
		if k > max {
			max = k
		}
	}
	for i := 1; i < max; i++ {
		if _, ok := args[i]; !ok {
			args[i] = makeSymbol("p")
		}
	}
}

func makeFnForm(args map[int]Symbol, body ReadObject) ReadObject {
	fillInMissingArgs(args)
	a := make([]Symbol, len(args))
	for key, value := range args {
		if key != -1 {
			a[key-1] = value
		}
	}
	if v, ok := args[-1]; ok {
		a[len(args)-1] = Symbol("&")
		a = append(a, v)
	}
	argVector := EmptyVector
	for _, v := range a {
		argVector = argVector.conj(v)
	}
	return DeriveReadObject(body, NewListFrom(Symbol("fn"), argVector, body))
}

func isTerminatingMacro(r rune) bool {
	switch r {
	case '"', ';', '@', '^', '`', '~', '(', ')', '[', ']', '{', '}', '\\', '%':
		return true
	default:
		return false
	}
}

func makeSymbol(prefix string) Symbol {
	GENSYM++
	return Symbol(fmt.Sprintf("%s__%d#", prefix, GENSYM))
}

func registerArg(index int) Symbol {
	if s, ok := ARGS[index]; ok {
		return s
	}
	ARGS[index] = makeSymbol("p")
	return ARGS[index]
}

func readArgSymbol(reader *Reader) ReadObject {
	r := reader.Peek()
	if unicode.IsSpace(r) || isTerminatingMacro(r) {
		return MakeReadObject(reader, registerArg(1))
	}
	obj := Read(reader)
	if obj.obj.Equals(Symbol("&")) {
		return MakeReadObject(reader, registerArg(-1))
	}
	switch n := obj.obj.(type) {
	case Int:
		return MakeReadObject(reader, registerArg(int(n)))
	default:
		panic(MakeReadError(reader, "Arg literal must be %, %& or %integer"))
	}
}

func isSelfEvaluating(obj Object) bool {
	if obj == EmptyList {
		return true
	}
	switch obj.(type) {
	case Bool, Double, Int, Char, Keyword, String:
		return true
	default:
		return false
	}
}

func isCall(obj Object, name Symbol) bool {
	switch seq := obj.(type) {
	case Seq:
		return seq.First().Equals(name)
	default:
		return false
	}
}

func syntaxQuoteSeq(seq Seq, env map[Symbol]Symbol, reader *Reader) Seq {
	res := make([]Object, 0)
	for iter := iter(seq); iter.HasNext(); {
		obj := iter.Next().(ReadObject)
		if isCall(obj.obj, Symbol("unquote-splicing")) {
			res = append(res, (obj.obj).(Seq).Rest().First())
		} else {
			q := makeSyntaxQuote(obj, env, reader)
			res = append(res, ReadObject{line: q.line, column: q.column, obj: NewListFrom(Symbol("list"), q)})
		}
	}
	return &ArraySeq{arr: res}
}

func syntaxQuoteColl(seq Seq, env map[Symbol]Symbol, reader *Reader, ctor Symbol, line int, column int) ReadObject {
	q := syntaxQuoteSeq(seq, env, reader)
	concat := q.Cons(Symbol("concat"))
	seqList := NewListFrom(Symbol("seq"), concat)
	if ctor == Symbol("") {
		return ReadObject{line: line, column: column, obj: seqList}
	}
	return ReadObject{line: line, column: column, obj: NewListFrom(ctor, seqList).Cons(Symbol("apply"))}
}

func makeSyntaxQuote(obj ReadObject, env map[Symbol]Symbol, reader *Reader) ReadObject {
	if isSelfEvaluating(obj.obj) {
		return obj
	}
	switch s := obj.obj.(type) {
	case Symbol:
		str := string(s)
		if r, _ := utf8.DecodeLastRuneInString(str); r == '#' {
			sym, ok := env[s]
			if !ok {
				sym = makeSymbol(str[:len(str)-1])
				env[s] = sym
			}
			obj = ReadObject{column: obj.column, line: obj.line, obj: sym}
		}
		return makeQuote(obj, Symbol("quote"))
	case Seq:
		if isCall(obj.obj, Symbol("unquote")) {
			return Second(s).(ReadObject)
		}
		if isCall(obj.obj, Symbol("unquote-splicing")) {
			panic(MakeReadError(reader, "Splice not in list"))
		}
		return syntaxQuoteColl(s, env, reader, Symbol(""), obj.line, obj.column)
	case *Vector:
		return syntaxQuoteColl(s.Seq(), env, reader, Symbol("vector"), obj.line, obj.column)
	case *ArrayMap:
		return syntaxQuoteColl(ArraySeqFromArrayMap(s), env, reader, Symbol("hash-map"), obj.line, obj.column)
	case *Set:
		return syntaxQuoteColl(s.Seq(), env, reader, Symbol("hash-set"), obj.line, obj.column)
	default:
		return obj
	}
}

func readTagged(reader *Reader) ReadObject {
	obj := Read(reader)
	switch s := obj.obj.(type) {
	case Symbol:
		readFunc := DATA_READERS[s]
		if readFunc == nil {
			panic(MakeReadError(reader, "No reader function for tag "+string(s)))
		}
		return readFunc(reader)
	default:
		panic(MakeReadError(reader, "Reader tag must be a symbol"))
	}
}

func readDispatch(reader *Reader) ReadObject {
	r := reader.Get()
	switch r {
	case '"':
		return readString(reader, true)
	case '\'':
		nextObj := Read(reader)
		return DeriveReadObject(nextObj, NewListFrom(DeriveReadObject(nextObj, Symbol("var")), nextObj))
	case '^':
		return readWithMeta(reader)
	case '{':
		return readSet(reader)
	case '(':
		reader.Unget()
		ARGS = make(map[int]Symbol)
		fn := Read(reader)
		res := makeFnForm(ARGS, fn)
		ARGS = nil
		return res
	}
	reader.Unget()
	return readTagged(reader)
}

func readWithMeta(reader *Reader) ReadObject {
	meta := readMeta(reader)
	nextObj := Read(reader)
	return makeWithMeta(nextObj, meta)
}

func Read(reader *Reader) ReadObject {
	eatWhitespace(reader)
	r := reader.Get()
	switch {
	case r == '\\':
		return readCharacter(reader)
	case unicode.IsDigit(r):
		reader.Unget()
		return readNumber(reader)
	case r == '-':
		if unicode.IsDigit(reader.Peek()) {
			reader.Unget()
			return readNumber(reader)
		}
		return readSymbol(reader, '-')
	case r == '%' && ARGS != nil:
		return readArgSymbol(reader)
	case isSymbolInitial(r):
		return readSymbol(reader, r)
	case r == '"':
		return readString(reader, false)
	case r == '(':
		return readList(reader)
	case r == '[':
		return readVector(reader)
	case r == '{':
		return readMap(reader)
	case r == '/' && isDelimiter(reader.Peek()):
		return MakeReadObject(reader, Symbol("/"))
	case r == '\'':
		nextObj := Read(reader)
		return makeQuote(nextObj, Symbol("quote"))
	case r == '@':
		nextObj := Read(reader)
		return DeriveReadObject(nextObj, NewListFrom(DeriveReadObject(nextObj, Symbol("deref")), nextObj))
	case r == '~':
		if reader.Peek() == '@' {
			reader.Get()
			nextObj := Read(reader)
			return makeQuote(nextObj, Symbol("unquote-splicing"))
		}
		nextObj := Read(reader)
		return makeQuote(nextObj, Symbol("unquote"))
	case r == '`':
		nextObj := Read(reader)
		return makeSyntaxQuote(nextObj, make(map[Symbol]Symbol), reader)
	case r == '^':
		return readWithMeta(reader)
	case r == '#':
		return readDispatch(reader)
	case r == EOF:
		panic(MakeReadError(reader, "Unexpected end of file"))
	}
	panic(MakeReadError(reader, fmt.Sprintf("Unexpected %c", r)))
}

func TryRead(reader *Reader) (obj ReadObject, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	eatWhitespace(reader)
	if reader.Peek() == EOF {
		return ReadObject{}, io.EOF
	}
	return Read(reader), nil
}
