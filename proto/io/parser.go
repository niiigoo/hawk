package io

import (
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"io"
	"regexp"
	"strings"
)

type Proto struct {
	Pos lexer.Position

	Entries []*Entry `parser:"( @@ ';'* )*"`
}

type Entry struct {
	Pos lexer.Position

	Syntax  string   `parser:"  'syntax' '=' @String"`
	Package string   `parser:"| 'package' @(Ident ( '.' Ident )*)"`
	Import  string   `parser:"| 'import' @String"`
	Message *Message `parser:"| @@"`
	Service *Service `parser:"| @@"`
	Enum    *Enum    `parser:"| @@"`
	Option  *Option  `parser:"| 'option' @@"`
	Extend  *Extend  `parser:"| @@"`
}

type Option struct {
	Pos lexer.Position

	Name  string  `parser:"( '(' @Ident @( '.' Ident )* ')' | @Ident @( '.' @Ident )* )"`
	Attr  *string `parser:"( '.' @Ident ( '.' @Ident )* )?"`
	Value *Value  `parser:"'=' @@"`
}

type Value struct {
	Pos lexer.Position

	String    *string  `parser:"  @String"`
	Number    *float64 `parser:"| @Float"`
	Int       *int64   `parser:"| @Int"`
	Bool      *bool    `parser:"| (@'true' | 'false')"`
	Reference *string  `parser:"| @Ident @( '.' Ident )*"`
	Map       *Map     `parser:"| @@"`
	Array     *Array   `parser:"| @@"`
}

type Array struct {
	Pos lexer.Position

	Elements []*Value `parser:"'[' ( @@ ( ','? @@ )* )? ']'"`
}

type Map struct {
	Pos lexer.Position

	Entries []*MapEntry `parser:"'{' ( @@ ( ( ',' )? @@ )* )? '}'"`
}

type MapEntry struct {
	Pos lexer.Position

	Key   *Value `parser:"@@"`
	Value *Value `parser:"':'? @@"`
}

type Extensions struct {
	Pos lexer.Position

	Extensions []Range `parser:"'extensions' @@ ( ',' @@ )*"`
}

type Reserved struct {
	Pos lexer.Position

	Reserved []Range `parser:"'reserved' @@ ( ',' @@ )*"`
}

type Range struct {
	Ident string `parser:"  @String"`
	Start int    `parser:"| ( @Int"`
	End   *int   `parser:"  ( 'to' ( @Int"`
	Max   bool   `parser:"           | @'max' ) )? )"`
}

type Extend struct {
	Pos lexer.Position

	Reference string   `parser:"'extend' @Ident ( '.' @Ident )*"`
	Fields    []*Field `parser:"'{' ( @@ ';'? )* '}'"`
}

type Service struct {
	Pos lexer.Position

	Name    string          `parser:"'service' @Ident"`
	Entries []*ServiceEntry `parser:"'{' ( @@ ';'? )* '}'"`
}

type ServiceEntry struct {
	Pos lexer.Position

	Option *Option `parser:"  'option' @@"`
	Method *Method `parser:"| @@"`
}

type Method struct {
	Pos lexer.Position

	Name              string    `parser:"'rpc' @Ident"`
	StreamingRequest  bool      `parser:"'(' @'stream'?"`
	Request           *Type     `parser:"    @@ ')'"`
	StreamingResponse bool      `parser:"'returns' '(' @'stream'?"`
	Response          *Type     `parser:"              @@ ')'"`
	Options           []*Option `parser:"( '{' ( 'option' @@ ';' )* '}' )?"`
}

type Enum struct {
	Pos lexer.Position

	Name   string       `parser:"'enum' @Ident"`
	Values []*EnumEntry `parser:"'{' ( @@ ( ';' )* )* '}'"`
}

type EnumEntry struct {
	Pos lexer.Position

	Value  *EnumValue `parser:"  @@"`
	Option *Option    `parser:"| 'option' @@"`
}

type EnumValue struct {
	Pos lexer.Position

	Key   string `parser:"@Ident"`
	Value int    `parser:"'=' @( [ '-' ] Int )"`

	Options []*Option `parser:"( '[' @@ ( ',' @@ )* ']' )?"`
}

type Message struct {
	Pos lexer.Position

	Name    string          `parser:"'message' @Ident"`
	Entries []*MessageEntry `parser:"'{' @@* '}'"`
}

type MessageEntry struct {
	Pos lexer.Position

	Enum       *Enum       `parser:"( @@"`
	Option     *Option     `parser:" | 'option' @@"`
	Message    *Message    `parser:" | @@"`
	OneOf      *OneOf      `parser:" | @@"`
	Extend     *Extend     `parser:" | @@"`
	Reserved   *Reserved   `parser:" | @@"`
	Extensions *Extensions `parser:" | @@"`
	Field      *Field      `parser:" | @@ ) ';'*"`
}

type OneOf struct {
	Pos lexer.Position

	Name    string        `parser:"'oneof' @Ident"`
	Entries []*OneOfEntry `parser:"'{' ( @@ ';'* )* '}'"`
}

type OneOfEntry struct {
	Pos lexer.Position

	Field  *Field  `parser:"  @@"`
	Option *Option `parser:"| 'option' @@"`
}

type Field struct {
	Pos lexer.Position

	Optional bool `parser:"(   @'optional'"`
	Required bool `parser:"  | @'required'"`
	Repeated bool `parser:"  | @'repeated' )?"`

	Type Type   `parser:"@@"`
	Name string `parser:"@Ident"`
	Tag  int    `parser:"'=' @Int"`

	Options []*Option `parser:"( '[' @@ ( ',' @@ )* ']' )?"`
}

type Scalar int

const (
	None Scalar = iota
	Double
	Float
	Int32
	Int64
	Uint32
	Uint64
	Sint32
	Sint64
	Fixed32
	Fixed64
	SFixed32
	SFixed64
	Bool
	String
	Bytes
)

var scalarToString = map[Scalar]string{
	None: "none", Double: "double", Float: "float", Int32: "int32", Int64: "int64", Uint32: "uint32",
	Uint64: "uint64", Sint32: "sint32", Sint64: "sint64", Fixed32: "fixed32", Fixed64: "fixed64",
	SFixed32: "sfixed32", SFixed64: "sfixed64", Bool: "bool", String: "string", Bytes: "bytes",
}

func (s *Scalar) GoString() string { return scalarToString[*s] }

var stringToScalar = map[string]Scalar{
	"double": Double, "float": Float, "int32": Int32, "int64": Int64, "uint32": Uint32, "uint64": Uint64,
	"sint32": Sint32, "sint64": Sint64, "fixed32": Fixed32, "fixed64": Fixed64, "sfixed32": SFixed32,
	"sfixed64": SFixed64, "bool": Bool, "string": String, "bytes": Bytes,
}

func (s *Scalar) Parse(lex *lexer.PeekingLexer) error {
	token := lex.Peek()
	v, ok := stringToScalar[token.Value]
	if !ok {
		return participle.NextMatch
	}
	lex.Next()
	*s = v
	return nil
}

type Type struct {
	Pos lexer.Position

	Scalar    Scalar   `parser:"  @@"`
	Map       *MapType `parser:"| @@"`
	Reference string   `parser:"| @(Ident ( '.' Ident )*)"`
}

type MapType struct {
	Pos lexer.Position

	Key   *Type `parser:"'map' '<' @@"`
	Value *Type `parser:"',' @@ '>'"`
}

type Path struct {
	Segments []Segment
	//SegmentsRaw []string `parser:"'/' (@Element|@Ident)+ ('/' (@Element|@Ident)+)* (?= (':' Ident)?)"`
	Verb *string `parser:"(':' (@Element|@Ident)+)?"`
}

type Segment struct {
	Wildcard *string
	Literal  *string
	Variable *Variable
}

type Variable struct {
	Field    string   `parser:"@Ident"`
	Segments []string `parser:"('=' @Ident ('/' @Ident)*)?"`
	Pattern  *string  `parser:"(':' (@Ident|@Element|@Symbol)+)?"`
}

var (
	parser = participle.MustBuild[Proto](participle.Unquote("String"), participle.UseLookahead(2))
	//parserPath = participle.MustBuild[Path](participle.Lexer(lexer.MustSimple([]lexer.SimpleRule{
	//	{"Ident", `[a-zA-Z_][a-zA-Z0-9_-]*`},
	//	{"Symbol", `[/:]`},
	//	{"Element", `[^/:]+`},
	//})))
	//parserPathSegment = participle.MustBuild[Variable](participle.Lexer(lexer.MustSimple([]lexer.SimpleRule{
	//	{"Ident", `[a-zA-Z_][a-zA-Z0-9_-]*`},
	//	{"Symbol", `[=:]`},
	//	{"Slash", `/`},
	//	{"Element", `[^/]+`},
	//})))
	patternVerb = regexp.MustCompile(`(.*):([A-Za-z]+)$`)
)

func Parse(filename string, r io.Reader) (*Proto, error) {
	return parser.Parse(filename, r)
}

func ParseString(filename string, data string) (*Proto, error) {
	return parser.ParseString(filename, data)
}

// TODO: improve this function
func ParsePath(data string) (*Path, error) {
	path := Path{
		Segments: make([]Segment, 0),
	}

	matches := patternVerb.FindStringSubmatch(data)
	if len(matches) > 0 {
		path.Verb = &matches[2]
		data = matches[1]
	}

	parts := strings.Split(strings.TrimPrefix(data, "/"), "/")
	for i := 0; i < len(parts); i++ {
		s := parts[i]

		if s == "*" || s == "**" {
			path.Segments = append(path.Segments, Segment{
				Wildcard: &s,
			})
		} else if strings.HasPrefix(s, "{") {
			v := Variable{
				Segments: make([]string, 0),
			}
			s = s[1:]
			pos := strings.Index(s, "=")
			segments := false
			if pos >= 0 {
				segments = true
				v.Field = s[:pos]
				s = s[pos+1:]
				for {
					pos = strings.Index(s, "}")
					if pos >= 0 {
						break
					}
					if len(s) > 0 {
						v.Segments = append(v.Segments, s)
					}
					i++
					s = parts[i]
				}
			}
			s = s[:len(s)-1]
			if len(s) > 0 {
				pos = strings.Index(s, ":")
				if pos < 0 {
					if segments {
						v.Segments = append(v.Segments, s)
					} else {
						v.Field = s
					}
				} else {
					if pos > 0 {
						if segments {
							v.Segments = append(v.Segments, s[:pos])
						} else {
							v.Field = s[:pos]
						}
					}
					p := s[pos+1:]
					v.Pattern = &p
				}
			}
			path.Segments = append(path.Segments, Segment{
				Variable: &v,
			})
		} else {
			path.Segments = append(path.Segments, Segment{
				Literal: &s,
			})
		}
	}

	return &path, nil
}
