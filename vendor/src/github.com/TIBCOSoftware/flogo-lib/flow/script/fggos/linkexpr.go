package fggos

import (
	"strings"

	"encoding/json"
	"strconv"

	"github.com/TIBCOSoftware/flogo-lib/core/data"
	"github.com/TIBCOSoftware/flogo-lib/flow/flowdef"
	"github.com/japm/goScript"
	"github.com/TIBCOSoftware/flogo-lib/logger"
)


// GosLinkExprManager is the Lua Implementation of a Link Expression Manager
type GosLinkExprManager struct {
	values map[int][]*varInfo
	exprs  map[int]*goScript.Expr
}

type varInfo struct {
	name string
	isd  int
}

// NewGosLinkExprManager creates a new LuaLinkExprManager
func NewGosLinkExprManager(def *flowdef.Definition) *GosLinkExprManager {

	mgr := &GosLinkExprManager{}
	mgr.values = make(map[int][]*varInfo)
	mgr.exprs = make(map[int]*goScript.Expr)

	links := flowdef.GetExpressionLinks(def)

	for _, link := range links {

		if len(strings.TrimSpace(link.Value())) > 0 {
			vars, exprStr := transExpr(link.Value())

			mgr.values[link.ID()] = vars

			logger.Debugf("expr: %v\n", exprStr)

			expr := &goScript.Expr{}
			err := expr.Prepare(exprStr)

			if err == nil {
				mgr.exprs[link.ID()] = expr
			} else {
				logger.Errorf("Error preparing expression: %s - %v", link.Value(), err)
			}
		}
	}

	return mgr
}

func transExpr(s string) ([]*varInfo, string) {

	var vars []*varInfo
	var rvars []string

	strLen := len(s)

	isd := 0

	for i := 0; i < strLen; i++ {
		if s[i] == '$' {

			isdefcheck := false

			if strings.HasSuffix(s[0:i], "isDefined(") {
				isdefcheck = true
			}

			ignoreBraces := s[i+1] == '{'
			var partOfName bool

			var j int
			for j = i + 1; j < strLen; j++ {

				partOfName, ignoreBraces = isPartOfName(s[j], ignoreBraces)

				if !partOfName {
					break
				}
			}

			if isdefcheck {
				isd++
				vars = append(vars, &varInfo{isd: isd, name: s[i+1 : j]})
				rvars = append(rvars, s[i-10:j+1])
				rvars = append(rvars, "isd"+strconv.Itoa(isd))
				i = j + 1
			} else {
				vars = append(vars, &varInfo{name: s[i+1 : j]})
				rvars = append(rvars, s[i:j])
				rvars = append(rvars, `v["`+s[i+1:j]+`"]`)
				i = j
			}
		}
	}

	replacer := strings.NewReplacer(rvars...)

	return vars, replacer.Replace(s)
}

func isPartOfName(char byte, ignoreBraces bool) (bool, bool) {

	if (char < '0' || char > '9') && (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && char != '.' && char != '_' {

		if ignoreBraces && char == '{' {
			return true, true
		} else if ignoreBraces && char == '}' {
			return true, false
		}

		return false, ignoreBraces

	}

	return true, ignoreBraces
}

// EvalLinkExpr implements LinkExprManager.EvalLinkExpr
func (em *GosLinkExprManager) EvalLinkExpr(link *flowdef.Link, scope data.Scope) bool {

	if link.Type() == flowdef.LtDependency {
		// dependency links are always true
		return true
	}

	vars, attrsOK := em.values[link.ID()]
	expr, exprOK := em.exprs[link.ID()]

	if !attrsOK || !exprOK {

		logger.Warnf("Unable to evaluate expression '%s', did not compile properly\n", link.Value())
		return false
	}

	ctxt := make(map[string]interface{})
	vals := make(map[string]interface{})

	for _, varInfo := range vars {

		var attrValue interface{}

		attrName, attrPath, _ := data.GetAttrPath(varInfo.name)
		attr, exists := scope.GetAttr(attrName)

		attrValue = attr.Value

		if varInfo.isd > 0 {

			if exists && len(attrPath) > 0 {

				//for now assume if we have a path, attr is "object" and only one level
				valMap := attrValue.(map[string]interface{})
				//todo what if the value does not exists
				_, exists = valMap[attrPath]
			}

			ctxt["isd"+strconv.Itoa(varInfo.isd)] = exists

		} else {

			if exists && len(attrPath) > 0 {

				val := data.GetMapValue(attrValue.(map[string]interface{}), attrPath)
				attrValue = FixUpValue(val)
			}

			vals[varInfo.name] = attrValue
		}
	}

	ctxt["v"] = vals

	logger.Debugf("Vals: %v", vals)

	val, err := expr.Eval(ctxt)

	//todo handle error
	if err != nil {
		logger.Error(err)
	}

	return val.(bool)
}

// FixUpValue fixes json numbers
func FixUpValue(val interface{}) interface{} {

	ret := val
	var err error

	switch t := val.(type) {
	case json.Number:
		if strings.Index(t.String(), ".") > -1 {
			ret, err = t.Float64()
		} else {
			ret, err = t.Int64()
		}
	}

	if err != nil {
		ret = val
	}

	return ret
}

