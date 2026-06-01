package bedrock

import (
	"strings"
	"testing"
)

func runRule(rule reviewRule, code, filename string) []violation {
	return rule(newLineIndex(code), filename)
}

// ── LineIndex（移植 line_index.rs tests）────────────────────────────────

func TestIndentOf(t *testing.T) {
	cases := map[string]int{"    foo": 4, "\tfoo": 4, "\t  foo": 6, "foo": 0}
	for in, want := range cases {
		if got := indentOf(in); got != want {
			t.Errorf("indentOf(%q)=%d want %d", in, got, want)
		}
	}
}

func TestStripComment(t *testing.T) {
	cases := [][2]string{
		{"x = 1 # hello", "x = 1 "},
		{"# nothing", ""},
		{"plain code", "plain code"},
		{`s = "a # not comment" # yes`, `s = "a # not comment" `},
		{"s = 'a # b' + c # comment", "s = 'a # b' + c "},
	}
	for _, c := range cases {
		if got := stripComment(c[0]); got != c[1] {
			t.Errorf("stripComment(%q)=%q want %q", c[0], got, c[1])
		}
	}
}

func TestDocstringMask(t *testing.T) {
	idx := newLineIndex("def f():\n    \"\"\"docs\n    spans\n    here\"\"\"\n    return 1\n")
	got := idx.docstringMask()
	want := []bool{false, true, true, true, false}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("multiline mask=%v want %v", got, want)
		}
	}
	idx2 := newLineIndex("x = 1\n\"\"\"oneliner\"\"\"\ny = 2\n")
	got2 := idx2.docstringMask()
	want2 := []bool{false, true, false}
	for i := range want2 {
		if got2[i] != want2[i] {
			t.Fatalf("single-line mask=%v want %v", got2, want2)
		}
	}
}

// ── 12 规则（移植各 rules/*.rs tests 的命中计数）────────────────────────

func TestRuleFstring(t *testing.T) {
	if v := runRule(ruleFstring, "name = 'a'\nx = f\"hello {name}\"\n", "t.py"); len(v) != 1 || v[0].line != 2 || v[0].severity != sevCritical {
		t.Fatalf("basic fstring: %+v", v)
	}
	if v := runRule(ruleFstring, "x = f'foo {bar}'\n", "t.py"); len(v) != 1 {
		t.Fatalf("single-quote: %d", len(v))
	}
	if v := runRule(ruleFstring, "x = \"hello\"\ny = 'world'\nz = format(1)\n", "t.py"); len(v) != 0 {
		t.Fatalf("plain strings should not flag: %d", len(v))
	}
	if v := runRule(ruleFstring, "feature = True\nformat_str = '{}'\n", "t.py"); len(v) != 0 {
		t.Fatalf("ident named f: %d", len(v))
	}
	if v := runRule(ruleFstring, "def x():\n    \"\"\"example f\"oops\" inside docs\"\"\"\n    return 1\n", "t.py"); len(v) != 0 {
		t.Fatalf("in docstring: %d", len(v))
	}
}

func TestRulePrintCall(t *testing.T) {
	if v := runRule(rulePrintCall, "print('hello')\n", "t.py"); len(v) != 1 || v[0].line != 1 {
		t.Fatalf("print without future: %+v", v)
	}
	if v := runRule(rulePrintCall, "from __future__ import print_function\nprint('hello')\n", "t.py"); len(v) != 0 {
		t.Fatalf("with future import: %d", len(v))
	}
	if v := runRule(rulePrintCall, "import pprint\npprint.print(x)\nself.print(x)\n", "t.py"); len(v) != 0 {
		t.Fatalf("pprint/attr: %d", len(v))
	}
	if v := runRule(rulePrintCall, "print 'hello'\n", "t.py"); len(v) != 0 {
		t.Fatalf("print statement (no paren): %d", len(v))
	}
}

func TestRuleTypeHints(t *testing.T) {
	if v := runRule(ruleTypeHints, "def f(x: int):\n    pass\n", "t.py"); len(v) != 1 || v[0].rule != "type_hints" {
		t.Fatalf("param annotation: %+v", v)
	}
	if v := runRule(ruleTypeHints, "def f(x) -> bool:\n    return True\n", "t.py"); len(v) != 1 {
		t.Fatalf("return annotation: %d", len(v))
	}
	if v := runRule(ruleTypeHints, "def f(x=1, y=None):\n    pass\n", "t.py"); len(v) != 0 {
		t.Fatalf("defaults: %d", len(v))
	}
	if v := runRule(ruleTypeHints, "d = {'a': 1, 'b': 2}\n", "t.py"); len(v) != 0 {
		t.Fatalf("dict literal: %d", len(v))
	}
	if v := runRule(ruleTypeHints, "x = lst[1:2]\n", "t.py"); len(v) != 0 {
		t.Fatalf("slice: %d", len(v))
	}
}

func TestRuleAsyncAwait(t *testing.T) {
	if v := runRule(ruleAsyncAwait, "async def f():\n    pass\n", "t.py"); len(v) != 1 {
		t.Fatalf("async def: %d", len(v))
	}
	if v := runRule(ruleAsyncAwait, "async def f():\n    await foo()\n", "t.py"); len(v) != 2 {
		t.Fatalf("async+await: %d", len(v))
	}
	if v := runRule(ruleAsyncAwait, "my_async_handler = None\nawaitable_var = 0\n", "t.py"); len(v) != 0 {
		t.Fatalf("ident: %d", len(v))
	}
}

func TestRuleWalrus(t *testing.T) {
	if v := runRule(ruleWalrus, "if (n := len(items)) > 10:\n    pass\n", "t.py"); len(v) != 1 {
		t.Fatalf("walrus: %d", len(v))
	}
	if v := runRule(ruleWalrus, "if x == 1:\n    pass\nif x != 2:\n    pass\nif x >= 1:\n    pass\nif x <= 2:\n    pass\n", "t.py"); len(v) != 0 {
		t.Fatalf("comparisons: %d", len(v))
	}
	if v := runRule(ruleWalrus, "d = {'a': 1}\n", "t.py"); len(v) != 0 {
		t.Fatalf("annotation colon: %d", len(v))
	}
}

func TestRuleCrossSide(t *testing.T) {
	if v := runRule(ruleCrossSide, "import mod.clientApi as clientApi\nclass Foo(ServerSystem):\n    pass\n", "FooServer.py"); len(v) != 1 || v[0].rule != "cross_side" {
		t.Fatalf("server imports client: %+v", v)
	}
	if v := runRule(ruleCrossSide, "from mod.serverApi import x\nclass Foo(ClientSystem):\n    pass\n", "FooClient.py"); len(v) != 1 {
		t.Fatalf("client imports server: %d", len(v))
	}
	if v := runRule(ruleCrossSide, "import mod.serverApi as serverApi\nclass Foo(ServerSystem):\n    pass\n", "FooServer.py"); len(v) != 0 {
		t.Fatalf("same side: %d", len(v))
	}
	if v := runRule(ruleCrossSide, "import mod.clientApi as clientApi\n", "helper.py"); len(v) != 0 {
		t.Fatalf("no side context: %d", len(v))
	}
	if v := runRule(ruleCrossSide, "import mod.clientApi as clientApi\n", "my_server_module.py"); len(v) != 1 {
		t.Fatalf("filename heuristic: %d", len(v))
	}
}

func TestRuleEngineFactory(t *testing.T) {
	if v := runRule(ruleEngineFactory, "class S(ServerSystem):\n    def foo(self):\n        cf = serverApi.GetEngineCompFactory()\n", "t.py"); len(v) != 1 {
		t.Fatalf("inside method: %d", len(v))
	}
	if v := runRule(ruleEngineFactory, "import mod.serverApi as serverApi\nCF = serverApi.GetEngineCompFactory()\n", "t.py"); len(v) != 0 {
		t.Fatalf("module level: %d", len(v))
	}
}

func TestRuleFnImport(t *testing.T) {
	if v := runRule(ruleFnImport, "def foo():\n    import os\n    return os\n", "t.py"); len(v) != 1 {
		t.Fatalf("inside function: %d", len(v))
	}
	if v := runRule(ruleFnImport, "class C(object):\n    def foo(self):\n        from os import path\n        return path\n", "t.py"); len(v) != 1 {
		t.Fatalf("inside method: %d", len(v))
	}
	if v := runRule(ruleFnImport, "import os\nfrom sys import argv\n\ndef foo():\n    return os\n", "t.py"); len(v) != 0 {
		t.Fatalf("module level: %d", len(v))
	}
	if v := runRule(ruleFnImport, "class C(object):\n    import os\n", "t.py"); len(v) != 0 {
		t.Fatalf("class body: %d", len(v))
	}
}

func TestRuleTickThrottle(t *testing.T) {
	if v := runRule(ruleTickThrottle, "def Tick(self):\n    a = 1\n    b = 2\n    c = 3\n    do(a, b, c)\n", "t.py"); len(v) != 1 {
		t.Fatalf("tick without throttle: %d", len(v))
	}
	if v := runRule(ruleTickThrottle, "def Tick(self):\n    self._n = (self._n + 1) % 11\n    if self._n:\n        return\n    do()\n", "t.py"); len(v) != 0 {
		t.Fatalf("with prime modulus: %d", len(v))
	}
	if v := runRule(ruleTickThrottle, "def OnTick(self):\n    pass\n", "t.py"); len(v) != 0 {
		t.Fatalf("short stub: %d", len(v))
	}
	if v := runRule(ruleTickThrottle, "def helper(self):\n    a = 1\n    b = 2\n    c = 3\n    do(a, b, c)\n", "t.py"); len(v) != 0 {
		t.Fatalf("non-tick: %d", len(v))
	}
}

func TestRuleBlockEntitySalt(t *testing.T) {
	if v := runRule(ruleBlockEntitySalt, "def handle(self, event):\n    # ServerBlockEntityTickEvent payload\n    do(event)\n    more(event)\n", "t.py"); len(v) != 1 {
		t.Fatalf("without salt: %d", len(v))
	}
	if v := runRule(ruleBlockEntitySalt, "def handle(self, event):\n    # ServerBlockEntityTickEvent payload\n    if hash((event['x'], event['y'])) % 11:\n        return\n    do(event)\n", "t.py"); len(v) != 0 {
		t.Fatalf("with hash: %d", len(v))
	}
	if v := runRule(ruleBlockEntitySalt, "def foo(self):\n    return 1\n", "t.py"); len(v) != 0 {
		t.Fatalf("unrelated: %d", len(v))
	}
}

func TestRuleMagicNumber(t *testing.T) {
	if v := runRule(ruleMagicNumber, "x = 777\n", "t.py"); len(v) != 1 || !strings.Contains(v[0].message, "777") {
		t.Fatalf("uncommon: %+v", v)
	}
	if v := runRule(ruleMagicNumber, "x = 256\ny = 1024\nz = 200\n", "t.py"); len(v) != 0 {
		t.Fatalf("common constants: %d", len(v))
	}
	if v := runRule(ruleMagicNumber, "x = 50\ny = 99\n", "t.py"); len(v) != 0 {
		t.Fatalf("small: %d", len(v))
	}
	if v := runRule(ruleMagicNumber, "a=111\nb=222\nc=333\nd=444\ne=555\n", "t.py"); len(v) != 3 {
		t.Fatalf("cap at 3: %d", len(v))
	}
	if v := runRule(ruleMagicNumber, "x = 100.5\ny = 3.14e10\n", "t.py"); len(v) != 0 {
		t.Fatalf("floats: %d", len(v))
	}
}

func TestRuleDictAccess(t *testing.T) {
	if v := runRule(ruleDictAccess, "name = data['user']\n", "t.py"); len(v) != 1 {
		t.Fatalf("direct subscript: %d", len(v))
	}
	if v := runRule(ruleDictAccess, "name = data.get('user')\n", "t.py"); len(v) != 0 {
		t.Fatalf("get call: %d", len(v))
	}
	if v := runRule(ruleDictAccess, "env = os.environ['HOME']\n", "t.py"); len(v) != 0 {
		t.Fatalf("excluded prefix: %d", len(v))
	}
	if v := runRule(ruleDictAccess, "a = d['a']\nb = d['b']\nc = d['c']\ne = d['e']\nf = d['f']\ng = d['g']\n", "t.py"); len(v) != 5 {
		t.Fatalf("cap at 5: %d", len(v))
	}
	if v := runRule(ruleDictAccess, "x = lst[0]\ny = lst[1]\n", "t.py"); len(v) != 0 {
		t.Fatalf("int index: %d", len(v))
	}
}

// ── 流水线 + 报告（移植 mod.rs tests）────────────────────────────────────

func TestReviewCode_Empty(t *testing.T) {
	r, _ := reviewCode("", "empty.py")
	if !strings.Contains(r, "Lines reviewed: 0") || !strings.Contains(r, "No issues found") {
		t.Fatalf("empty:\n%s", r)
	}
}

func TestReviewCode_CleanModule(t *testing.T) {
	src := "from __future__ import print_function\n" +
		"import mod.serverApi as serverApi\n" +
		"CF = serverApi.GetEngineCompFactory()\n" +
		"\n" +
		"class S(object):\n" +
		"    def helper(self):\n" +
		"        value = self.data.get('count', 0)\n" +
		"        print('ok', value)\n" +
		"        return value\n"
	r, _ := reviewCode(src, "clean.py")
	if !strings.Contains(r, "No issues found") {
		t.Fatalf("clean module should have no issues:\n%s", r)
	}
}

func TestReviewCode_FullHouse(t *testing.T) {
	src := "import mod.clientApi as clientApi\n" +
		"class S(ServerSystem):\n" +
		"    def Tick(self):\n" +
		"        import os\n" +
		"        cf = serverApi.GetEngineCompFactory()\n" +
		"        name = data['user']\n" +
		"        print(\"hello\")\n" +
		"        x = f\"value {name}\"\n" +
		"        if (n := len(name)) > 1:\n" +
		"            y = 777\n" +
		"        a = b\n" +
		"    async def go(self) -> bool:\n" +
		"        await self.helper()\n" +
		"        return True\n" +
		"    def OnBlockTick(self, event):\n" +
		"        # ServerBlockEntityTickEvent payload\n" +
		"        do(event)\n" +
		"        more(event)\n"
	r, _ := reviewCode(src, "BadServer.py")
	for _, sec := range []string{"## Critical", "## Warning", "## Suggestion"} {
		if !strings.Contains(r, sec) {
			t.Fatalf("缺分段 %s:\n%s", sec, r)
		}
	}
	for _, rule := range []string{
		"fstring", "print_call", "type_hints", "async_await", "walrus", "cross_side",
		"engine_factory", "fn_import", "tick_throttle", "block_entity_salt",
		"magic_number", "dict_access",
	} {
		if !strings.Contains(r, rule) {
			t.Fatalf("缺规则 %s:\n%s", rule, r)
		}
	}
	// 报告头 + 排序：Critical 段在 Warning 段前。
	if !strings.HasPrefix(r, "# Review Report: BadServer.py\n") {
		t.Fatalf("报告头不符:\n%.60s", r)
	}
	if strings.Index(r, "## Critical") > strings.Index(r, "## Warning") {
		t.Fatalf("Critical 段应在 Warning 段前:\n%s", r)
	}
}

func TestReviewCode_FilenameDefault(t *testing.T) {
	svc := New()
	r, _ := svc.ReviewCode("x = 1\n", nil) // filename 省略 → unknown.py
	if !strings.Contains(r, "# Review Report: unknown.py") {
		t.Fatalf("默认 filename 应为 unknown.py:\n%.60s", r)
	}
	fn := "custom.py"
	r2, _ := svc.ReviewCode("x = 1\n", &fn)
	if !strings.Contains(r2, "# Review Report: custom.py") {
		t.Fatalf("filename 应透传 custom.py:\n%.60s", r2)
	}
}
