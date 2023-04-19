package io

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type ParserTestSuite struct {
	suite.Suite
}

func TestParserTestSuite(t *testing.T) {
	suite.Run(t, new(ParserTestSuite))
}

func (s *ParserTestSuite) TestParsePath_Literals() {
	path := `/v1/entity`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Empty(p.Verb)
	s.Require().Len(p.Segments, 2)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
}

func (s *ParserTestSuite) TestParsePath_Verb() {
	path := `/v1/entity:VERB`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Equal("VERB", *p.Verb)
	s.Require().Len(p.Segments, 2)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
}

func (s *ParserTestSuite) TestParsePath_VariableSimple() {
	path := `/v1/entity/{id}`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Empty(p.Verb)
	s.Require().Len(p.Segments, 3)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
	s.Equal("id", p.Segments[2].Variable.Field)
}

func (s *ParserTestSuite) TestParsePath_VariableSegments() {
	path := `/v1/entity/{id=v1/test}/abc`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Empty(p.Verb)
	s.Require().Len(p.Segments, 4)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
	s.Equal("id", p.Segments[2].Variable.Field)
	s.Require().Len(p.Segments[2].Variable.Segments, 2)
	s.Equal("abc", *p.Segments[3].Literal)
}

func (s *ParserTestSuite) TestParsePath_VariablePatternSimple() {
	path := `/v1/entity/{id:[0-9]+}`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Empty(p.Verb)
	s.Require().Len(p.Segments, 3)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
	s.Equal("id", p.Segments[2].Variable.Field)
	s.Equal("[0-9]+", *p.Segments[2].Variable.Pattern)
	s.Empty(p.Segments[2].Variable.Segments)
}

func (s *ParserTestSuite) TestParsePath_VariablePattern() {
	path := `/v1/entity/{id=v1/test:[0-9]+[a-z]{1,3}}/abc`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Empty(p.Verb)
	s.Require().Len(p.Segments, 4)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
	s.Equal("id", p.Segments[2].Variable.Field)
	s.Equal("[0-9]+[a-z]{1,3}", *p.Segments[2].Variable.Pattern)
	s.Require().Len(p.Segments[2].Variable.Segments, 2)
	s.Equal("abc", *p.Segments[3].Literal)
}

func (s *ParserTestSuite) TestParsePath_Wildcards() {
	path := `/v1/entity/*/abc/**`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Empty(p.Verb)
	s.Require().Len(p.Segments, 5)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
	s.Equal("*", *p.Segments[2].Wildcard)
	s.Equal("abc", *p.Segments[3].Literal)
	s.Equal("**", *p.Segments[4].Wildcard)
}

func (s *ParserTestSuite) TestParsePath_Complex() {
	path := `/v1/entity/*/{id=v1/test}/**:VERB`

	p, err := ParsePath(path)

	s.Require().NoError(err)
	s.Equal("VERB", *p.Verb)
	s.Require().Len(p.Segments, 5)
	s.Equal("v1", *p.Segments[0].Literal)
	s.Equal("entity", *p.Segments[1].Literal)
	s.Equal("*", *p.Segments[2].Wildcard)
	s.Equal("id", p.Segments[3].Variable.Field)
	s.Equal("**", *p.Segments[4].Wildcard)
	s.Require().Len(p.Segments[3].Variable.Segments, 2)
}
