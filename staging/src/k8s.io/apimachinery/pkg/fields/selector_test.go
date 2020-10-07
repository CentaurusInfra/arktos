/*
Copyright 2015 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

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

package fields

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitTermsAndSort(t *testing.T) {
	testcases := map[string][][]string{
		// Simple selectors
		`a`:                                                   {{`a`}},
		`a=avalue`:                                            {{`a=avalue`}},
		`a=avalue,b=bvalue`:                                   {{`a=avalue`, `b=bvalue`}},
		`a=avalue,b==bvalue,c!=cvalue`:                        {{`a=avalue`, `b==bvalue`, `c!=cvalue`}},
		`a=gt:avalue,b==lt:bvalue`:                            {{`a=gt:avalue`, `b==lt:bvalue`}},
		`a=gte:avalue,b==lte:bvalue`:                          {{`a=gte:avalue`, `b==lte:bvalue`}},
		`a=gte:avalue;b==lte:bvalue`:                          {{`a=gte:avalue`}, {`b==lte:bvalue`}},
		`a=gt:avalue;b==lt:bvalue;c==lt:cvalue`:               {{`a=gt:avalue`}, {`b==lt:bvalue`}, {`c==lt:cvalue`}},
		`a=gte:avalue;b==lte:bvalue,c==lt:cvalue`:             {{`a=gte:avalue`}, {`b==lte:bvalue`, `c==lt:cvalue`}},
		`a=gte:avalue,b==lte:bvalue;c==lt:cvalue`:             {{`a=gte:avalue`, `b==lte:bvalue`}, {`c==lt:cvalue`}},
		`a=gte:avalue,b==lte:bvalue;c==lt:cvalue,d=gt:dvalue`: {{`a=gte:avalue`, `b==lte:bvalue`}, {`c==lt:cvalue`, `d=gt:dvalue`}},

		// Empty terms
		``:         nil,
		`a=a,`:     {{``, `a=a`}},
		`a=a;`:     {nil, {`a=a`}},
		`,a=a`:     {{``, `a=a`}},
		`;a=a`:     {nil, {`a=a`}},
		`a=gt:a,`:  {{``, `a=gt:a`}},
		`a=gt:a;`:  {nil, {`a=gt:a`}},
		`,a=gt:a`:  {{``, `a=gt:a`}},
		`;a=gt:a`:  {nil, {`a=gt:a`}},
		`a=gte:a,`: {{``, `a=gte:a`}},
		`a=gte:a;`: {nil, {`a=gte:a`}},
		`,a=gte:a`: {{``, `a=gte:a`}},
		`;a=gte:a`: {nil, {`a=gte:a`}},
		`a=lt:a,`:  {{``, `a=lt:a`}},
		`a=lt:a;`:  {nil, {`a=lt:a`}},
		`,a=lt:a`:  {{``, `a=lt:a`}},
		`;a=lt:a`:  {nil, {`a=lt:a`}},
		`a=lte:a,`: {{``, `a=lte:a`}},
		`a=lte:a;`: {nil, {`a=lte:a`}},
		`,a=lte:a`: {{``, `a=lte:a`}},
		`;a=lte:a`: {nil, {`a=lte:a`}},

		// Escaped values
		`k=\,,k2=v2`:   {{`k2=v2`, `k=\,`}},     // escaped comma in value
		`k=\\,k2=v2`:   {{`k2=v2`, `k=\\`}},     // escaped backslash, unescaped comma
		`k=\\\,,k2=v2`: {{`k2=v2`, `k=\\\,`}},   // escaped backslash and comma
		`k=\;;k2=v2`:   {{`k2=v2`}, {`k=\;`}},   // escaped comma in value
		`k=\\;k2=v2`:   {{`k2=v2`}, {`k=\\`}},   // escaped backslash, unescaped comma
		`k=\\\;;k2=v2`: {{`k2=v2`}, {`k=\\\;`}}, // escaped backslash and comma
		`k=\a\b\`:      {{`k=\a\b\`}},           // non-escape sequences
		`k=\`:          {{`k=\`}},               // orphan backslash

		// Great than Escaped values
		`k=\,,k2=gt:v2`:   {{`k2=gt:v2`, `k=\,`}},     // escaped comma in value
		`k=\\,k2=gt:v2`:   {{`k2=gt:v2`, `k=\\`}},     // escaped backslash, unescaped comma
		`k=\\\,,k2=gt:v2`: {{`k2=gt:v2`, `k=\\\,`}},   // escaped backslash and comma
		`k=\;;k2=gt:v2`:   {{`k2=gt:v2`}, {`k=\;`}},   // escaped comma in value
		`k=\\;k2=gt:v2`:   {{`k2=gt:v2`}, {`k=\\`}},   // escaped backslash, unescaped comma
		`k=\\\;;k2=gt:v2`: {{`k2=gt:v2`}, {`k=\\\;`}}, // escaped backslash and comma
		`k=gt:\a\b\`:      {{`k=gt:\a\b\`}},           // non-escape sequences
		`k=gt:\`:          {{`k=gt:\`}},               // orphan backslash
		// Great than or equals Escaped values
		`k=\,,k2=gte:v2`:   {{`k2=gte:v2`, `k=\,`}},     // escaped comma in value
		`k=\\,k2=gte:v2`:   {{`k2=gte:v2`, `k=\\`}},     // escaped backslash, unescaped comma
		`k=\\\,,k2=gte:v2`: {{`k2=gte:v2`, `k=\\\,`}},   // escaped backslash and comma
		`k=\;;k2=gte:v2`:   {{`k2=gte:v2`}, {`k=\;`}},   // escaped comma in value
		`k=\\;k2=gte:v2`:   {{`k2=gte:v2`}, {`k=\\`}},   // escaped backslash, unescaped comma
		`k=\\\;;k2=gte:v2`: {{`k2=gte:v2`}, {`k=\\\;`}}, // escaped backslash and comma
		`k=gte:\a\b\`:      {{`k=gte:\a\b\`}},           // non-escape sequences
		`k=gte:\`:          {{`k=gte:\`}},               // orphan backslash

		// Less than Escaped values
		`k=\,,k2=lt:v2`:   {{`k2=lt:v2`, `k=\,`}},     // escaped comma in value
		`k=\\,k2=lt:v2`:   {{`k2=lt:v2`, `k=\\`}},     // escaped backslash, unescaped comma
		`k=\\\,,k2=lt:v2`: {{`k2=lt:v2`, `k=\\\,`}},   // escaped backslash and comma
		`k=\;;k2=lt:v2`:   {{`k2=lt:v2`}, {`k=\;`}},   // escaped comma in value
		`k=\\;k2=lt:v2`:   {{`k2=lt:v2`}, {`k=\\`}},   // escaped backslash, unescaped comma
		`k=\\\;;k2=lt:v2`: {{`k2=lt:v2`}, {`k=\\\;`}}, // escaped backslash and comma
		`k=lt:\a\b\`:      {{`k=lt:\a\b\`}},           // non-escape sequences
		`k=lt:\`:          {{`k=lt:\`}},               // orphan backslash

		// Less than or equals Escaped values
		`k=\,,k2=lte:v2`:   {{`k2=lte:v2`, `k=\,`}},     // escaped comma in value
		`k=\\,k2=lte:v2`:   {{`k2=lte:v2`, `k=\\`}},     // escaped backslash, unescaped comma
		`k=\\\,,k2=lte:v2`: {{`k2=lte:v2`, `k=\\\,`}},   // escaped backslash and comma
		`k=\;;k2=lte:v2`:   {{`k2=lte:v2`}, {`k=\;`}},   // escaped comma in value
		`k=\\;k2=lte:v2`:   {{`k2=lte:v2`}, {`k=\\`}},   // escaped backslash, unescaped comma
		`k=\\\;;k2=lte:v2`: {{`k2=lte:v2`}, {`k=\\\;`}}, // escaped backslash and comma
		`k=lte:\a\b\`:      {{`k=lte:\a\b\`}},           // non-escape sequences
		`k=lte:\`:          {{`k=lte:\`}},               // orphan backslash

		// Multi-byte
		`함=수,목=록`: {{`목=록`, `함=수`}},
	}

	for selector, expectedTerms := range testcases {
		if terms := splitTermsAndSort(selector); !reflect.DeepEqual(terms, expectedTerms) {
			t.Errorf("splitSelectors(`%s`): Expected\n%#v\ngot\n%#v", selector, expectedTerms, terms)
		}
	}
}

func TestSplitTerm(t *testing.T) {
	testcases := map[string]struct {
		lhs string
		op  string
		rhs string
		ok  bool
	}{
		// Simple terms
		`a=value`:     {lhs: `a`, op: `=`, rhs: `value`, ok: true},
		`b==value`:    {lhs: `b`, op: `==`, rhs: `value`, ok: true},
		`c!=value`:    {lhs: `c`, op: `!=`, rhs: `value`, ok: true},
		`d=lt:value`:  {lhs: `d`, op: `=lt:`, rhs: `value`, ok: true},
		`e=lte:value`: {lhs: `e`, op: `=lte:`, rhs: `value`, ok: true},
		`f=gt:value`:  {lhs: `f`, op: `=gt:`, rhs: `value`, ok: true},
		`g=gte:value`: {lhs: `g`, op: `=gte:`, rhs: `value`, ok: true},

		// Empty or invalid terms
		``:  {lhs: ``, op: ``, rhs: ``, ok: false},
		`a`: {lhs: ``, op: ``, rhs: ``, ok: false},

		// Escaped values
		`k=\,`:          {lhs: `k`, op: `=`, rhs: `\,`, ok: true},
		`k=\;`:          {lhs: `k`, op: `=`, rhs: `\;`, ok: true},
		`k=\=`:          {lhs: `k`, op: `=`, rhs: `\=`, ok: true},
		`k=\\\a\b\=\,\`: {lhs: `k`, op: `=`, rhs: `\\\a\b\=\,\`, ok: true},
		`k=\\\a\b\=\;\`: {lhs: `k`, op: `=`, rhs: `\\\a\b\=\;\`, ok: true},

		`k=gt:\,`:          {lhs: `k`, op: `=gt:`, rhs: `\,`, ok: true},
		`k=gt:\;`:          {lhs: `k`, op: `=gt:`, rhs: `\;`, ok: true},
		`k=gt:\=`:          {lhs: `k`, op: `=gt:`, rhs: `\=`, ok: true},
		`k=gt:\\\a\b\=\,\`: {lhs: `k`, op: `=gt:`, rhs: `\\\a\b\=\,\`, ok: true},
		`k=gt:\\\a\b\=\;\`: {lhs: `k`, op: `=gt:`, rhs: `\\\a\b\=\;\`, ok: true},

		`k=gte:\,`:          {lhs: `k`, op: `=gte:`, rhs: `\,`, ok: true},
		`k=gte:\;`:          {lhs: `k`, op: `=gte:`, rhs: `\;`, ok: true},
		`k=gte:\=`:          {lhs: `k`, op: `=gte:`, rhs: `\=`, ok: true},
		`k=gte:\\\a\b\=\,\`: {lhs: `k`, op: `=gte:`, rhs: `\\\a\b\=\,\`, ok: true},
		`k=gte:\\\a\b\=\;\`: {lhs: `k`, op: `=gte:`, rhs: `\\\a\b\=\;\`, ok: true},

		`k=lt:\,`:          {lhs: `k`, op: `=lt:`, rhs: `\,`, ok: true},
		`k=lt:\;`:          {lhs: `k`, op: `=lt:`, rhs: `\;`, ok: true},
		`k=lt:\=`:          {lhs: `k`, op: `=lt:`, rhs: `\=`, ok: true},
		`k=lt:\\\a\b\=\,\`: {lhs: `k`, op: `=lt:`, rhs: `\\\a\b\=\,\`, ok: true},
		`k=lt:\\\a\b\=\;\`: {lhs: `k`, op: `=lt:`, rhs: `\\\a\b\=\;\`, ok: true},

		`k=lte:\,`:          {lhs: `k`, op: `=lte:`, rhs: `\,`, ok: true},
		`k=lte:\;`:          {lhs: `k`, op: `=lte:`, rhs: `\;`, ok: true},
		`k=lte:\=`:          {lhs: `k`, op: `=lte:`, rhs: `\=`, ok: true},
		`k=lte:\\\a\b\=\,\`: {lhs: `k`, op: `=lte:`, rhs: `\\\a\b\=\,\`, ok: true},
		`k=lte:\\\a\b\=\;\`: {lhs: `k`, op: `=lte:`, rhs: `\\\a\b\=\;\`, ok: true},

		// Multi-byte
		`함=수`: {lhs: `함`, op: `=`, rhs: `수`, ok: true},
	}

	for term, expected := range testcases {
		lhs, op, rhs, ok := splitTerm(term)
		if lhs != expected.lhs || op != expected.op || rhs != expected.rhs || ok != expected.ok {
			t.Errorf(
				"splitTerm(`%s`): Expected\n%s,%s,%s,%v\nGot\n%s,%s,%s,%v",
				term,
				expected.lhs, expected.op, expected.rhs, expected.ok,
				lhs, op, rhs, ok,
			)
		}
	}
}

func TestEscapeValue(t *testing.T) {
	// map values to their normalized escaped values
	testcases := map[string]string{
		``:          ``,
		`a`:         `a`,
		`=`:         `\=`,
		`=gt:`:      `\=gt:`,
		`=gte:`:     `\=gte:`,
		`=lt:`:      `\=lt:`,
		`=lte:`:     `\=lte:`,
		`,`:         `\,`,
		`;`:         `\;`,
		`\`:         `\\`,
		`\=\,\`:     `\\\=\\\,\\`,
		`\=\;\`:     `\\\=\\\;\\`,
		`\=gt:\,\`:  `\\\=gt:\\\,\\`,
		`\=gt:\;\`:  `\\\=gt:\\\;\\`,
		`\=gte:\,\`: `\\\=gte:\\\,\\`,
		`\=gte:\;\`: `\\\=gte:\\\;\\`,
		`\=lt:\,\`:  `\\\=lt:\\\,\\`,
		`\=lt:\;\`:  `\\\=lt:\\\;\\`,
		`\=lte:\,\`: `\\\=lte:\\\,\\`,
		`\=lte:\;\`: `\\\=lte:\\\;\\`,
	}

	for unescapedValue, escapedValue := range testcases {
		actualEscaped := EscapeValue(unescapedValue)
		if actualEscaped != escapedValue {
			t.Errorf("EscapeValue(%s): expected %s, got %s", unescapedValue, escapedValue, actualEscaped)
		}

		actualUnescaped, err := UnescapeValue(escapedValue)
		if err != nil {
			t.Errorf("UnescapeValue(%s): unexpected error %v", escapedValue, err)
		}
		if actualUnescaped != unescapedValue {
			t.Errorf("UnescapeValue(%s): expected %s, got %s", escapedValue, unescapedValue, actualUnescaped)
		}
	}

	// test invalid escape sequences
	invalidTestcases := []string{
		`\`,   // orphan slash is invalid
		`\\\`, // orphan slash is invalid
		`\a`,  // unrecognized escape sequence is invalid
	}
	for _, invalidValue := range invalidTestcases {
		_, err := UnescapeValue(invalidValue)
		if _, ok := err.(InvalidEscapeSequence); !ok || err == nil {
			t.Errorf("UnescapeValue(%s): expected invalid escape sequence error, got %#v", invalidValue, err)
		}
	}
}

func TestSelectorParse(t *testing.T) {
	testGoodStrings := []string{
		"x=a,y=b,z=c",
		"x=a;y=b;z=c",
		"x=a,y=b;z=c",
		"",
		"x!=a,y=b",
		"x!=a;y=b",
		`x=a||y\=b`,
		`x=a\=\=b`,
	}
	testBadStrings := []string{
		"x=a||y=b",
		"x==a==b",
		"x=a,b",
		"x=gtt:a||y=ltt:b",
		"x=gt:a=lt:b",
		"x=gt:a,b",
		"x in (a)",
		"x in (a,b,c)",
		"x",
	}
	testConvertedStrings := [][]string{
		{"x=gt:a", "x>a"},
		{"x=gte:a", "x>=a"},
		{"x=lt:a", "x<a"},
		{"x=lte:a", "x<=a"},
		{"x=lt:a,y=lte:b,z=gt:c,zz=gte:d", "x<a,y<=b,z>c,zz>=d"},
		{"x=lt:a;y=lte:b;z=gt:c;zz=gte:d", "x<a;y<=b;z>c;zz>=d"},
		{"x=lt:a;y=lte:b,z=gt:c;zz=gte:d", "x<a;y<=b,z>c;zz>=d"},
		{`x=gt:a||y\=gt:b`, `x>a||y\=gt:b`},
		{`x=lt:a\=\=b`, `x<a\=\=b`},
	}
	for _, test := range testGoodStrings {
		lq, err := ParseSelector(test)
		if err != nil {
			t.Errorf("%v: error %v (%#v)\n", test, err, err)
		}
		if test != lq.String() {
			t.Errorf("%v restring gave: %v\n", test, lq.String())
		}
	}
	for _, test := range testBadStrings {
		_, err := ParseSelector(test)
		if err == nil {
			t.Errorf("%v: did not get expected error\n", test)
		}
	}
	for _, test := range testConvertedStrings {
		result, err := ParseSelector(test[0])

		if err != nil {
			t.Errorf("%v: did not get expected error\n", err)
		}
		if strings.Compare(result.String(), test[1]) != 0 {
			t.Errorf("The result %v does not match the expected result %v\n", result, test[1])
		}
	}
}

func TestDeterministicParse(t *testing.T) {
	s1, err := ParseSelector("x=a,a=x")
	s2, err2 := ParseSelector("a=x,x=a")
	if err != nil || err2 != nil {
		t.Errorf("Unexpected parse error")
	}
	if s1.String() != s2.String() {
		t.Errorf("Non-deterministic parse")
	}

	s1, err = ParseSelector("x=a;a=x")
	s2, err2 = ParseSelector("a=x;x=a")
	if err != nil || err2 != nil {
		t.Errorf("Unexpected parse error")
	}
	if s1.String() != s2.String() {
		t.Errorf("Non-deterministic parse")
	}
}

func expectMatch(t *testing.T, selector string, ls Set) {
	lq, err := ParseSelector(selector)
	if err != nil {
		t.Errorf("Unable to parse %v as a selector\n", selector)
		return
	}
	if !lq.Matches(ls) {
		t.Errorf("Wanted %s to match '%s', but it did not.\n", selector, ls)
	}
}

func expectNoMatch(t *testing.T, selector string, ls Set) {
	lq, err := ParseSelector(selector)
	if err != nil {
		t.Errorf("Unable to parse %v as a selector\n", selector)
		return
	}
	if lq.Matches(ls) {
		t.Errorf("Wanted '%s' to not match '%s', but it did.", selector, ls)
	}
}

func TestEverything(t *testing.T) {
	if !Everything().Matches(Set{"x": "y"}) {
		t.Errorf("Nil selector didn't match")
	}
	if !Everything().Empty() {
		t.Errorf("Everything was not empty")
	}
}

func TestSelectorMatches(t *testing.T) {
	expectMatch(t, "", Set{"x": "y"})
	expectMatch(t, "x=y", Set{"x": "y"})
	expectMatch(t, "x=y,z=w", Set{"x": "y", "z": "w"})
	expectMatch(t, "x!=y,z!=w", Set{"x": "z", "z": "a"})
	expectMatch(t, "notin=in", Set{"notin": "in"}) // in and notin in exactMatch
	expectNoMatch(t, "x=y", Set{"x": "z"})
	expectNoMatch(t, "x=y,z=w", Set{"x": "w", "z": "w"})
	expectNoMatch(t, "x!=y,z!=w", Set{"x": "z", "z": "w"})

	fieldset := Set{
		"foo":     "bar",
		"baz":     "blah",
		"complex": `=value\,\`,
	}
	expectMatch(t, "foo=bar", fieldset)
	expectMatch(t, "baz=blah", fieldset)
	expectMatch(t, "foo=bar,baz=blah", fieldset)
	expectMatch(t, `foo=bar,baz=blah,complex=\=value\\\,\\`, fieldset)
	expectNoMatch(t, "foo=blah", fieldset)
	expectNoMatch(t, "baz=bar", fieldset)
	expectNoMatch(t, "foo=bar,foobar=bar,baz=blah", fieldset)
}

func TestOneTermEqualSelector(t *testing.T) {
	if !OneTermEqualSelector("x", "y").Matches(Set{"x": "y"}) {
		t.Errorf("No match when match expected.")
	}
	if OneTermEqualSelector("x", "y").Matches(Set{"x": "z"}) {
		t.Errorf("Match when none expected.")
	}
}

func expectMatchDirect(t *testing.T, selector, ls Set) {
	if !SelectorFromSet(selector).Matches(ls) {
		t.Errorf("Wanted %s to match '%s', but it did not.\n", selector, ls)
	}
}

func expectNoMatchDirect(t *testing.T, selector, ls Set) {
	if SelectorFromSet(selector).Matches(ls) {
		t.Errorf("Wanted '%s' to not match '%s', but it did.", selector, ls)
	}
}

func TestSetMatches(t *testing.T) {
	labelset := Set{
		"foo": "bar",
		"baz": "blah",
	}
	expectMatchDirect(t, Set{}, labelset)
	expectMatchDirect(t, Set{"foo": "bar"}, labelset)
	expectMatchDirect(t, Set{"baz": "blah"}, labelset)
	expectMatchDirect(t, Set{"foo": "bar", "baz": "blah"}, labelset)
	expectNoMatchDirect(t, Set{"foo": "=blah"}, labelset)
	expectNoMatchDirect(t, Set{"baz": "=bar"}, labelset)
	expectNoMatchDirect(t, Set{"foo": "=bar", "foobar": "bar", "baz": "blah"}, labelset)
}

func TestNilMapIsValid(t *testing.T) {
	selector := Set(nil).AsSelector()
	if selector == nil {
		t.Errorf("Selector for nil set should be Everything")
	}
	if !selector.Empty() {
		t.Errorf("Selector for nil set should be Empty")
	}
}

func TestSetIsEmpty(t *testing.T) {
	if !(Set{}).AsSelector().Empty() {
		t.Errorf("Empty set should be empty")
	}
	if !(andTerm(nil)).Empty() {
		t.Errorf("Nil andTerm should be empty")
	}
	if !(orTerm(nil)).Empty() {
		t.Errorf("Nil orTerm should be empty")
	}
	if (&hasTerm{}).Empty() {
		t.Errorf("hasTerm should not be empty")
	}
	if (&notHasTerm{}).Empty() {
		t.Errorf("notHasTerm should not be empty")
	}
	if !(andTerm{andTerm{}}).Empty() {
		t.Errorf("Nested andTerm should be empty")
	}
	if !(orTerm{orTerm{}}).Empty() {
		t.Errorf("Nested orTerm should be empty")
	}
	if !(orTerm{andTerm{}}).Empty() {
		t.Errorf("Nested or/andTerm should be empty")
	}
	if (andTerm{&hasTerm{"a", "b"}}).Empty() {
		t.Errorf("Nested andTerm should not be empty")
	}
	if (orTerm{&hasTerm{"a", "b"}}).Empty() {
		t.Errorf("Nested orTerm should not be empty")
	}
}

func TestRequiresExactMatch(t *testing.T) {
	testCases := map[string]struct {
		S     Selector
		Label string
		Value string
		Found bool
	}{
		"empty set":                       {Set{}.AsSelector(), "test", "", false},
		"empty hasTerm":                   {&hasTerm{}, "test", "", false},
		"skipped hasTerm":                 {&hasTerm{"a", "b"}, "test", "", false},
		"valid hasTerm":                   {&hasTerm{"test", "b"}, "test", "b", true},
		"valid hasTerm no value":          {&hasTerm{"test", ""}, "test", "", true},
		"valid notHasTerm":                {&notHasTerm{"test", "b"}, "test", "", false},
		"valid notHasTerm no value":       {&notHasTerm{"test", ""}, "test", "", false},
		"nil andTerm":                     {andTerm(nil), "test", "", false},
		"empty andTerm":                   {andTerm{}, "test", "", false},
		"nested andTerm":                  {andTerm{andTerm{}}, "test", "", false},
		"nested andTerm matches":          {andTerm{&hasTerm{"test", "b"}}, "test", "b", true},
		"andTerm with non-match":          {andTerm{&hasTerm{}, &hasTerm{"test", "b"}}, "test", "b", true},
		"nil orTerm":                      {orTerm(nil), "test", "", false},
		"empty orTerm":                    {orTerm{}, "test", "", false},
		"nested orTerm":                   {orTerm{orTerm{}}, "test", "", false},
		"nested orTerm matches":           {orTerm{&hasTerm{"test", "b"}}, "test", "b", true},
		"orTerm with non-match":           {orTerm{&hasTerm{}, &hasTerm{"test", "b"}}, "test", "b", true},
		"nested or/andTerm":               {orTerm{andTerm{}}, "test", "", false},
		"empty lessTerm":                  {&lessTerm{}, "test", "", false},
		"skipped lessTerm":                {&lessTerm{"a", "b"}, "test", "", false},
		"valid lessTerm":                  {&lessTerm{"test", "b"}, "test", "", false},
		"valid lessTerm no value":         {&lessTerm{"test", ""}, "test", "", false},
		"empty lessEqualTerm":             {&lessEqualTerm{}, "test", "", false},
		"skipped lessEqualTerm":           {&lessEqualTerm{"a", "b"}, "test", "", false},
		"valid lessEqualTerm":             {&lessEqualTerm{"test", "b"}, "test", "", false},
		"valid lessEqualTerm no value":    {&lessEqualTerm{"test", ""}, "test", "", false},
		"empty greaterTerm":               {&greaterTerm{}, "test", "", false},
		"skipped greaterTerm":             {&greaterTerm{"a", "b"}, "test", "", false},
		"valid greaterTerm":               {&greaterTerm{"test", "b"}, "test", "", false},
		"valid greaterTerm no value":      {&greaterTerm{"test", ""}, "test", "", false},
		"empty greaterEqualTerm":          {&greaterEqualTerm{}, "test", "", false},
		"skipped greaterEqualTerm":        {&greaterEqualTerm{"a", "b"}, "test", "", false},
		"valid greaterEqualTerm":          {&greaterEqualTerm{"test", "b"}, "test", "", false},
		"valid greaterEqualTerm no value": {&greaterEqualTerm{"test", ""}, "test", "", false},
	}
	for k, v := range testCases {
		value, found := v.S.RequiresExactMatch(v.Label)
		if value != v.Value {
			t.Errorf("%s: expected value %s, got %s", k, v.Value, value)
		}
		if found != v.Found {
			t.Errorf("%s: expected found %t, got %t", k, v.Found, found)
		}
	}
}

func TestTransform(t *testing.T) {
	testCases := []struct {
		name      string
		selector  string
		transform func(field, value string) (string, string, error)
		result    string
		isEmpty   bool
	}{
		{
			name:      "empty selector",
			selector:  "",
			transform: func(field, value string) (string, string, error) { return field, value, nil },
			result:    "",
			isEmpty:   true,
		},
		{
			name:      "no-op transform andTerm",
			selector:  "a=b,c=d",
			transform: func(field, value string) (string, string, error) { return field, value, nil },
			result:    "a=b,c=d",
			isEmpty:   false,
		},
		{
			name:      "no-op transform orTerm",
			selector:  "a=b;c=d",
			transform: func(field, value string) (string, string, error) { return field, value, nil },
			result:    "a=b;c=d",
			isEmpty:   false,
		},
		{
			name:      "no-op transform orTerm, lessTerm, and greatTerm",
			selector:  "a=gt:b;c=lt:d",
			transform: func(field, value string) (string, string, error) { return field, value, nil },
			result:    "a>b;c<d",
			isEmpty:   false,
		},
		{
			name:      "no-op transform orTerm, lessEqualsTerm, and greatEqualsTerm",
			selector:  "a=gte:b;c=lte:d",
			transform: func(field, value string) (string, string, error) { return field, value, nil },
			result:    "a>=b;c<=d",
			isEmpty:   false,
		},
		{
			name:      "no-op transform orTerm & andTerm",
			selector:  "x=y,a=b;c=d",
			transform: func(field, value string) (string, string, error) { return field, value, nil },
			result:    "c=d;a=b,x=y",
			isEmpty:   false,
		},
		{
			name:     "transform one field andTerm",
			selector: "a=b,c=d",
			transform: func(field, value string) (string, string, error) {
				if field == "a" {
					return "e", "f", nil
				}
				return field, value, nil
			},
			result:  "e=f,c=d",
			isEmpty: false,
		},
		{
			name:     "transform one field orTerm",
			selector: "a=b;c=d",
			transform: func(field, value string) (string, string, error) {
				if field == "a" {
					return "e", "f", nil
				}
				return field, value, nil
			},
			result:  "e=f;c=d",
			isEmpty: false,
		},
		{
			name:      "remove field to make empty",
			selector:  "a=b",
			transform: func(field, value string) (string, string, error) { return "", "", nil },
			result:    "",
			isEmpty:   true,
		},
		{
			name:     "remove only one field andTerm",
			selector: "a=b,c=d,e=f",
			transform: func(field, value string) (string, string, error) {
				if field == "c" {
					return "", "", nil
				}
				return field, value, nil
			},
			result:  "a=b,e=f",
			isEmpty: false,
		},
		{
			name:     "remove only one field orTerm",
			selector: "a=b;c=d;e=f",
			transform: func(field, value string) (string, string, error) {
				if field == "c" {
					return "", "", nil
				}
				return field, value, nil
			},
			result:  "a=b;e=f",
			isEmpty: false,
		},
	}

	for i, tc := range testCases {
		result, err := ParseAndTransformSelector(tc.selector, tc.transform)
		if err != nil {
			t.Errorf("[%d] unexpected error during Transform: %v", i, err)
		}
		if result.Empty() != tc.isEmpty {
			t.Errorf("[%d] expected empty: %t, got: %t", i, tc.isEmpty, result.Empty())
		}
		if result.String() != tc.result {
			t.Errorf("[%d] unexpected result: %s", i, result.String())
		}
	}
}
