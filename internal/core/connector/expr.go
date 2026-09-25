package connector

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ภาษา expression เล็ก ๆ สำหรับ filter / not_found / model_context / ค่าที่คำนวณ
//
//	ตัวเลข · 'string' · "string" · true false null · ชื่อ (a, item.status, input.username, a[0])
//	+ - * / %  == != < <= > >=  && || !  (…)
//	ฟังก์ชัน: len(x) count(x) empty(x) sum(x,"f") min(a,b) max(a,b) abs(x) round(x,n)
//	          coalesce(a,b,…) contains(list,v) in(v,list…) lower(s)
//
// ไม่มี side effect · ไม่มี loop · ประเมินเสร็จในเวลาจำกัดเสมอ

type exprNode interface {
	eval(env map[string]any) (any, error)
}

// Expr = expression ที่ parse แล้ว (parse ครั้งเดียวตอนโหลด connector)
type Expr struct {
	src  string
	root exprNode
}

func ParseExpr(src string) (*Expr, error) {
	p := &exprParser{toks: nil}
	if err := p.lex(src); err != nil {
		return nil, err
	}
	n, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.toks) {
		return nil, fmt.Errorf("expr %q: ตัวอักษรเกินที่ %q", src, p.toks[p.pos].val)
	}
	return &Expr{src: src, root: n}, nil
}

func (e *Expr) Eval(env map[string]any) (any, error) { return e.root.eval(env) }

func (e *Expr) String() string { return e.src }

// ---------- lexer ----------

type tokKind int

const (
	tNum tokKind = iota
	tStr
	tIdent
	tOp
	tLParen
	tRParen
	tLBrack
	tRBrack
	tComma
	tDot
)

type token struct {
	kind tokKind
	val  string
}

type exprParser struct {
	toks []token
	pos  int
}

func (p *exprParser) lex(s string) error {
	r := []rune(s)
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case unicode.IsDigit(c):
			j := i
			for j < len(r) && (unicode.IsDigit(r[j]) || r[j] == '.') {
				j++
			}
			p.toks = append(p.toks, token{tNum, string(r[i:j])})
			i = j
		case c == '\'' || c == '"':
			j := i + 1
			var b strings.Builder
			for j < len(r) && r[j] != c {
				if r[j] == '\\' && j+1 < len(r) {
					j++
				}
				b.WriteRune(r[j])
				j++
			}
			if j >= len(r) {
				return fmt.Errorf("expr %q: string ไม่ปิด", s)
			}
			p.toks = append(p.toks, token{tStr, b.String()})
			i = j + 1
		case unicode.IsLetter(c) || c == '_':
			j := i
			for j < len(r) && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_') {
				j++
			}
			p.toks = append(p.toks, token{tIdent, string(r[i:j])})
			i = j
		case c == '(':
			p.toks = append(p.toks, token{tLParen, "("})
			i++
		case c == ')':
			p.toks = append(p.toks, token{tRParen, ")"})
			i++
		case c == '[':
			p.toks = append(p.toks, token{tLBrack, "["})
			i++
		case c == ']':
			p.toks = append(p.toks, token{tRBrack, "]"})
			i++
		case c == ',':
			p.toks = append(p.toks, token{tComma, ","})
			i++
		case c == '.':
			p.toks = append(p.toks, token{tDot, "."})
			i++
		default:
			two := ""
			if i+1 < len(r) {
				two = string(r[i : i+2])
			}
			switch two {
			case "==", "!=", "<=", ">=", "&&", "||":
				p.toks = append(p.toks, token{tOp, two})
				i += 2
				continue
			}
			switch c {
			case '+', '-', '*', '/', '%', '<', '>', '!':
				p.toks = append(p.toks, token{tOp, string(c)})
				i++
			default:
				return fmt.Errorf("expr %q: ตัวอักษร %q ใช้ไม่ได้", s, string(c))
			}
		}
	}
	return nil
}

func (p *exprParser) peek() *token {
	if p.pos < len(p.toks) {
		return &p.toks[p.pos]
	}
	return nil
}

func (p *exprParser) next() *token {
	t := p.peek()
	if t != nil {
		p.pos++
	}
	return t
}

var precedence = map[string]int{
	"||": 1, "&&": 2,
	"==": 3, "!=": 3,
	"<": 4, "<=": 4, ">": 4, ">=": 4,
	"+": 5, "-": 5,
	"*": 6, "/": 6, "%": 6,
}

func (p *exprParser) parseExpr(minPrec int) (exprNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t == nil || t.kind != tOp {
			return left, nil
		}
		prec, ok := precedence[t.val]
		if !ok || prec < minPrec {
			return left, nil
		}
		p.next()
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &binNode{op: t.val, l: left, r: right}
	}
}

func (p *exprParser) parseUnary() (exprNode, error) {
	t := p.peek()
	if t != nil && t.kind == tOp && (t.val == "!" || t.val == "-") {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &unaryNode{op: t.val, x: x}, nil
	}
	return p.parsePostfix()
}

func (p *exprParser) parsePostfix() (exprNode, error) {
	n, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t == nil {
			return n, nil
		}
		switch t.kind {
		case tDot:
			p.next()
			id := p.next()
			if id == nil || id.kind != tIdent {
				return nil, fmt.Errorf("หลัง . ต้องเป็นชื่อ field")
			}
			n = &memberNode{x: n, key: id.val}
		case tLBrack:
			p.next()
			idx, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			if c := p.next(); c == nil || c.kind != tRBrack {
				return nil, fmt.Errorf("ขาด ]")
			}
			n = &indexNode{x: n, idx: idx}
		default:
			return n, nil
		}
	}
}

func (p *exprParser) parsePrimary() (exprNode, error) {
	t := p.next()
	if t == nil {
		return nil, fmt.Errorf("expression จบก่อนเวลา")
	}
	switch t.kind {
	case tNum:
		f, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return nil, fmt.Errorf("ตัวเลข %q ผิด", t.val)
		}
		return &litNode{v: f}, nil
	case tStr:
		return &litNode{v: t.val}, nil
	case tLParen:
		n, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if c := p.next(); c == nil || c.kind != tRParen {
			return nil, fmt.Errorf("ขาด )")
		}
		return n, nil
	case tIdent:
		switch t.val {
		case "true":
			return &litNode{v: true}, nil
		case "false":
			return &litNode{v: false}, nil
		case "null", "nil":
			return &litNode{v: nil}, nil
		}
		if nt := p.peek(); nt != nil && nt.kind == tLParen {
			p.next()
			var args []exprNode
			if c := p.peek(); c != nil && c.kind == tRParen {
				p.next()
			} else {
				for {
					a, err := p.parseExpr(0)
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					c := p.next()
					if c == nil {
						return nil, fmt.Errorf("ขาด )")
					}
					if c.kind == tRParen {
						break
					}
					if c.kind != tComma {
						return nil, fmt.Errorf("คั่น argument ด้วย , เท่านั้น")
					}
				}
			}
			if _, ok := exprFuncs[t.val]; !ok {
				return nil, fmt.Errorf("ไม่รู้จักฟังก์ชัน %q", t.val)
			}
			return &callNode{fn: t.val, args: args}, nil
		}
		return &identNode{name: t.val}, nil
	}
	return nil, fmt.Errorf("ไม่คาดว่าจะเจอ %q", t.val)
}

// ---------- nodes ----------

type litNode struct{ v any }

func (n *litNode) eval(map[string]any) (any, error) { return n.v, nil }

type identNode struct{ name string }

func (n *identNode) eval(env map[string]any) (any, error) { return env[n.name], nil }

type memberNode struct {
	x   exprNode
	key string
}

func (n *memberNode) eval(env map[string]any) (any, error) {
	v, err := n.x.eval(env)
	if err != nil {
		return nil, err
	}
	if m, ok := v.(map[string]any); ok {
		return m[n.key], nil
	}
	return nil, nil
}

type indexNode struct{ x, idx exprNode }

func (n *indexNode) eval(env map[string]any) (any, error) {
	v, err := n.x.eval(env)
	if err != nil {
		return nil, err
	}
	i, err := n.idx.eval(env)
	if err != nil {
		return nil, err
	}
	switch c := v.(type) {
	case []any:
		f, ok := ToFloat(i)
		if !ok {
			return nil, nil
		}
		k := int(f)
		if k < 0 {
			k = len(c) + k
		}
		if k < 0 || k >= len(c) {
			return nil, nil
		}
		return c[k], nil
	case map[string]any:
		return c[fmt.Sprint(i)], nil
	}
	return nil, nil
}

type unaryNode struct {
	op string
	x  exprNode
}

func (n *unaryNode) eval(env map[string]any) (any, error) {
	v, err := n.x.eval(env)
	if err != nil {
		return nil, err
	}
	if n.op == "!" {
		return !Truthy(v), nil
	}
	f, ok := ToFloat(v)
	if !ok {
		return nil, fmt.Errorf("ใช้ - กับค่าที่ไม่ใช่ตัวเลข")
	}
	return -f, nil
}

type binNode struct {
	op   string
	l, r exprNode
}

func (n *binNode) eval(env map[string]any) (any, error) {
	l, err := n.l.eval(env)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "&&":
		if !Truthy(l) {
			return false, nil
		}
		r, err := n.r.eval(env)
		if err != nil {
			return nil, err
		}
		return Truthy(r), nil
	case "||":
		if Truthy(l) {
			return true, nil
		}
		r, err := n.r.eval(env)
		if err != nil {
			return nil, err
		}
		return Truthy(r), nil
	}
	r, err := n.r.eval(env)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "==":
		return looseEqual(l, r), nil
	case "!=":
		return !looseEqual(l, r), nil
	}
	lf, lok := ToFloat(l)
	rf, rok := ToFloat(r)
	if n.op == "+" && (!lok || !rok) {
		ls, lsok := l.(string)
		rs, rsok := r.(string)
		if lsok && rsok {
			return ls + rs, nil
		}
	}
	if !lok || !rok {
		// ค่าหายไป (null) ในเลขคณิต = ผลเป็น null ไม่ใช่ 0 — กันตัวเลขปลอมโผล่
		switch n.op {
		case "<", "<=", ">", ">=":
			return false, nil
		}
		return nil, nil
	}
	switch n.op {
	case "+":
		return lf + rf, nil
	case "-":
		return lf - rf, nil
	case "*":
		return lf * rf, nil
	case "/":
		if rf == 0 {
			return nil, nil
		}
		return lf / rf, nil
	case "%":
		if rf == 0 {
			return nil, nil
		}
		return math.Mod(lf, rf), nil
	case "<":
		return lf < rf, nil
	case "<=":
		return lf <= rf, nil
	case ">":
		return lf > rf, nil
	case ">=":
		return lf >= rf, nil
	}
	return nil, fmt.Errorf("operator %q ใช้ไม่ได้", n.op)
}

type callNode struct {
	fn   string
	args []exprNode
}

func (n *callNode) eval(env map[string]any) (any, error) {
	vals := make([]any, len(n.args))
	for i, a := range n.args {
		v, err := a.eval(env)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	return exprFuncs[n.fn](vals)
}

var exprFuncs = map[string]func([]any) (any, error){
	"len":   func(a []any) (any, error) { return float64(lenOf(arg(a, 0))), nil },
	"count": func(a []any) (any, error) { return float64(lenOf(arg(a, 0))), nil },
	"empty": func(a []any) (any, error) {
		v := arg(a, 0)
		if v == nil {
			return true, nil
		}
		switch x := v.(type) {
		case string:
			return strings.TrimSpace(x) == "", nil
		case []any, map[string]any:
			return lenOf(x) == 0, nil
		}
		return false, nil
	},
	"sum": func(a []any) (any, error) {
		list, _ := arg(a, 0).([]any)
		field, _ := arg(a, 1).(string)
		total := 0.0
		for _, it := range list {
			v := it
			if field != "" {
				v = GetPath(it, field)
			}
			if f, ok := ToFloat(v); ok {
				total += f
			}
		}
		return total, nil
	},
	"min": func(a []any) (any, error) { return minmax(a, true), nil },
	"max": func(a []any) (any, error) { return minmax(a, false), nil },
	"abs": func(a []any) (any, error) {
		f, ok := ToFloat(arg(a, 0))
		if !ok {
			return nil, nil
		}
		return math.Abs(f), nil
	},
	"round": func(a []any) (any, error) {
		f, ok := ToFloat(arg(a, 0))
		if !ok {
			return nil, nil
		}
		d, _ := ToFloat(arg(a, 1))
		p := math.Pow(10, d)
		return math.Round(f*p) / p, nil
	},
	"coalesce": func(a []any) (any, error) {
		for _, v := range a {
			if v != nil {
				return v, nil
			}
		}
		return nil, nil
	},
	"contains": func(a []any) (any, error) {
		list, _ := arg(a, 0).([]any)
		for _, it := range list {
			if looseEqual(it, arg(a, 1)) {
				return true, nil
			}
		}
		if s, ok := arg(a, 0).(string); ok {
			if sub, ok := arg(a, 1).(string); ok {
				return strings.Contains(s, sub), nil
			}
		}
		return false, nil
	},
	"in": func(a []any) (any, error) {
		if len(a) == 0 {
			return false, nil
		}
		for _, v := range a[1:] {
			if looseEqual(a[0], v) {
				return true, nil
			}
		}
		return false, nil
	},
	"lower": func(a []any) (any, error) {
		s, _ := arg(a, 0).(string)
		return strings.ToLower(s), nil
	},
}

func arg(a []any, i int) any {
	if i < len(a) {
		return a[i]
	}
	return nil
}

func lenOf(v any) int {
	switch x := v.(type) {
	case []any:
		return len(x)
	case map[string]any:
		return len(x)
	case string:
		return len([]rune(x))
	}
	return 0
}

func minmax(a []any, min bool) any {
	var best *float64
	consider := func(v any) {
		if f, ok := ToFloat(v); ok {
			if best == nil || (min && f < *best) || (!min && f > *best) {
				ff := f
				best = &ff
			}
		}
	}
	for _, v := range a {
		if list, ok := v.([]any); ok {
			for _, it := range list {
				consider(it)
			}
			continue
		}
		consider(v)
	}
	if best == nil {
		return nil
	}
	return *best
}

// ---------- helpers (ใช้ร่วมทั้ง package) ----------

// ToFloat แปลงตัวเลขทุกชนิด (รวม string ตัวเลข) เป็น float64
func ToFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case string:
		s := strings.ReplaceAll(strings.TrimSpace(x), ",", "")
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
	return 0, false
}

func Truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	if f, ok := ToFloat(v); ok {
		return f != 0
	}
	return true
}

func looseEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	af, aok := ToFloat(a)
	bf, bok := ToFloat(b)
	if aok && bok {
		return af == bf
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}
