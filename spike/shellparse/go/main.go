package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

type In struct {
	ID  int    `json:"id"`
	Cmd string `json:"cmd"`
}
type Out struct {
	ID    int      `json:"id"`
	OK    bool     `json:"ok"`
	Exes  []string `json:"exes"`
	Micro int64    `json:"micros"`
}

var wrappers = map[string]bool{"sudo": true, "doas": true, "env": true, "nohup": true, "time": true, "command": true, "exec": true, "timeout": true, "nice": true, "xargs": true, "builtin": true}
var flagWithArg = map[string]map[string]bool{
	"sudo":    {"-u": true, "-g": true, "-h": true, "-p": true, "-C": true},
	"timeout": {"-s": true, "-k": true},
	"nice":    {"-n": true},
	"xargs":   {"-I": true, "-n": true, "-P": true, "-L": true, "-s": true, "-d": true},
	"env":     {"-u": true, "-C": true},
}

func wordStr(w *syntax.Word) string {
	for _, p := range w.Parts {
		switch p.(type) {
		case *syntax.ParamExp, *syntax.CmdSubst, *syntax.ArithmExp, *syntax.ProcSubst:
			return "<dyn>"
		}
		if dq, ok := p.(*syntax.DblQuoted); ok {
			for _, q := range dq.Parts {
				switch q.(type) {
				case *syntax.ParamExp, *syntax.CmdSubst, *syntax.ArithmExp:
					return "<dyn>"
				}
			}
		}
	}
	s, err := expand.Literal(&expand.Config{}, w)
	if err != nil {
		return "<dyn>"
	}
	return s
}

func isAssign(s string) bool {
	i := strings.Index(s, "=")
	return i > 0 && !strings.ContainsAny(s[:i], "/ -")
}

func extract(src string, depth int, out *[]string) bool {
	if depth > 4 {
		return true
	}
	p := syntax.NewParser(syntax.Variant(syntax.LangBash))
	f, err := p.Parse(strings.NewReader(src), "")
	if err != nil {
		return false
	}
	syntax.Walk(f, func(n syntax.Node) bool {
		c, ok := n.(*syntax.CallExpr)
		if !ok || len(c.Args) == 0 {
			return true
		}
		args := make([]string, len(c.Args))
		for i, a := range c.Args {
			args[i] = wordStr(a)
		}
		handle(args, depth, out)
		return true
	})
	return true
}

func handle(args []string, depth int, out *[]string) {
	i := 0
	for i < len(args) && isAssign(args[i]) {
		i++
	}
	if i >= len(args) {
		return
	}
	name := args[i]
	rest := args[i+1:]
	base := name
	if wrappers[base] {
		j := 0
		for j < len(rest) {
			a := rest[j]
			if base == "timeout" && len(a) > 0 && a[0] >= '0' && a[0] <= '9' {
				j++
				continue
			}
			if strings.HasPrefix(a, "-") && a != "-" {
				if flagWithArg[base][a] {
					j += 2
				} else {
					j++
				}
				continue
			}
			if isAssign(a) && (base == "env" || base == "sudo") {
				j++
				continue
			}
			break
		}
		if j < len(rest) {
			handle(rest[j:], depth, out)
		}
		return
	}
	if (base == "bash" || base == "sh" || base == "zsh" || base == "dash") && len(rest) > 0 {
		for k, a := range rest {
			if a == "-c" && k+1 < len(rest) {
				if rest[k+1] == "<dyn>" {
					*out = append(*out, "<dyn>")
				} else {
					extract(rest[k+1], depth+1, out)
				}
				return
			}
		}
	}
	if base == "find" {
		*out = append(*out, "find")
		for k, a := range rest {
			if (a == "-exec" || a == "-execdir") && k+1 < len(rest) {
				handle(rest[k+1:], depth, out)
			}
		}
		return
	}
	*out = append(*out, name)
}

func main() {
	var ins []In
	b, _ := os.ReadFile(os.Args[1])
	json.Unmarshal(b, &ins)
	var outs []Out
	for _, in := range ins {
		t := time.Now()
		var exes []string
		ok := extract(in.Cmd, 0, &exes)
		outs = append(outs, Out{ID: in.ID, OK: ok, Exes: exes, Micro: time.Since(t).Microseconds()})
	}
	json.NewEncoder(os.Stdout).Encode(outs)
}
