package handlers

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// FormulaEvaluator 公式计算器结构体
type FormulaEvaluator struct {
	formula    string
	variables  map[string]float64
	thresholds map[string]float64
}

func normalizeFormulaText(formula string) string {
	return strings.Join(strings.Fields(formula), "")
}

// NewFormulaEvaluator 创建新的公式计算器
func NewFormulaEvaluator(formula string) *FormulaEvaluator {
	return &FormulaEvaluator{
		formula:    normalizeFormulaText(formula),
		variables:  make(map[string]float64),
		thresholds: make(map[string]float64),
	}
}

// SetVariable 设置变量值
func (fe *FormulaEvaluator) SetVariable(name string, value float64) {
	fe.variables[name] = value
}

// SetVariables 批量设置变量值
func (fe *FormulaEvaluator) SetVariables(variables map[string]float64) {
	for k, v := range variables {
		fe.SetVariable(k, v)
	}
}

// SetThreshold 设置变量阈值
func (fe *FormulaEvaluator) SetThreshold(name string, threshold float64) {
	fe.thresholds[name] = threshold
}

// SetThresholds 批量设置变量阈值
func (fe *FormulaEvaluator) SetThresholds(thresholds map[string]float64) {
	for k, v := range thresholds {
		fe.thresholds[k] = v
	}
}

// Validate 验证公式语法
func (fe *FormulaEvaluator) Validate() error {
	// 检查括号是否匹配
	if !fe.checkParentheses() {
		return errors.New("括号不匹配")
	}

	// 检查公式是否为空
	if strings.TrimSpace(fe.formula) == "" {
		return errors.New("公式不能为空")
	}

	return nil
}

// checkParentheses 检查括号是否匹配
func (fe *FormulaEvaluator) checkParentheses() bool {
	count := 0
	for _, char := range fe.formula {
		switch char {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

// Evaluate 计算公式结果
func (fe *FormulaEvaluator) Evaluate() (float64, error) {
	// 验证公式
	if err := fe.Validate(); err != nil {
		return 0, err
	}

	// 先处理需要保留基因名的阈值函数，再替换变量为实际值
	formula := fe.evaluateNamedFunctions(fe.formula)
	formula = fe.replaceVariables(formula)

	// 计算结果
	result, err := fe.calculate(formula)
	if err != nil {
		return 0, err
	}

	return result, nil
}

// replaceVariables 替换变量为实际值
func (fe *FormulaEvaluator) replaceVariables(formula string) string {
	// 替换变量
	for varName, value := range fe.variables {
		// 使用正则表达式确保只替换完整的变量名
		re := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(varName)))
		formula = re.ReplaceAllString(formula, strconv.FormatFloat(value, 'f', -1, 64))
	}

	return formula
}

// evaluateNamedFunctions 处理需要基因名称才能查阈值的函数
func (fe *FormulaEvaluator) evaluateNamedFunctions(expression string) string {
	re := regexp.MustCompile(`threshold\(([^()]+)\)`)
	for re.MatchString(expression) {
		matches := re.FindStringSubmatch(expression)
		if len(matches) < 2 {
			break
		}

		geneName := strings.TrimSpace(matches[1])
		paramValue, exists := fe.variables[geneName]
		if !exists {
			if value, err := strconv.ParseFloat(geneName, 64); err == nil {
				paramValue = value
				exists = true
			}
		}
		if !exists {
			break
		}

		if threshold, ok := fe.thresholds[geneName]; ok && paramValue < threshold {
			expression = strings.Replace(expression, matches[0], "0", 1)
		} else {
			expression = strings.Replace(expression, matches[0], strconv.FormatFloat(paramValue, 'f', -1, 64), 1)
		}
	}

	re = regexp.MustCompile(`count_ge_threshold\(([^()]+)\)`)
	for re.MatchString(expression) {
		matches := re.FindStringSubmatch(expression)
		if len(matches) < 2 {
			break
		}

		count := 0
		for _, arg := range splitFormulaArgs(matches[1]) {
			geneName := strings.TrimSpace(arg)
			if geneName == "" {
				continue
			}
			value, exists := fe.variables[geneName]
			if !exists {
				if parsed, err := strconv.ParseFloat(geneName, 64); err == nil {
					value = parsed
					exists = true
				}
			}
			if !exists {
				continue
			}
			threshold := fe.thresholds[geneName]
			if value >= threshold {
				count++
			}
		}
		expression = strings.Replace(expression, matches[0], strconv.Itoa(count), 1)
	}

	return expression
}

// calculate 计算表达式结果
func (fe *FormulaEvaluator) calculate(expression string) (float64, error) {
	// 处理函数
	expression = fe.evaluateFunctions(expression)

	result, err := parseArithmeticExpression(expression)
	if err != nil {
		return 0, errors.New("公式格式错误")
	}

	return result, nil
}

type arithmeticParser struct {
	input string
	pos   int
}

func parseArithmeticExpression(input string) (float64, error) {
	parser := &arithmeticParser{input: input}
	value, err := parser.parseExpression()
	if err != nil {
		return 0, err
	}
	parser.skipSpaces()
	if parser.pos != len(parser.input) {
		return 0, fmt.Errorf("unexpected token at %d", parser.pos)
	}
	return value, nil
}

func (p *arithmeticParser) skipSpaces() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n' || p.input[p.pos] == '\r') {
		p.pos++
	}
}

func (p *arithmeticParser) parseExpression() (float64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.input) || (p.input[p.pos] != '+' && p.input[p.pos] != '-') {
			return value, nil
		}
		operator := p.input[p.pos]
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if operator == '+' {
			value += right
		} else {
			value -= right
		}
	}
}

func (p *arithmeticParser) parseTerm() (float64, error) {
	value, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.input) || (p.input[p.pos] != '*' && p.input[p.pos] != '/') {
			return value, nil
		}
		operator := p.input[p.pos]
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if operator == '*' {
			value *= right
			continue
		}
		if right == 0 {
			return 0, errors.New("除数不能为零")
		}
		value /= right
	}
}

func (p *arithmeticParser) parseFactor() (float64, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return 0, errors.New("公式格式错误")
	}
	sign := 1.0
	for p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		if p.input[p.pos] == '-' {
			sign *= -1
		}
		p.pos++
		p.skipSpaces()
	}
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		p.pos++
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, errors.New("括号表达式格式错误")
		}
		p.pos++
		return sign * value, nil
	}

	start := p.pos
	hasDigit := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch >= '0' && ch <= '9' {
			hasDigit = true
			p.pos++
			continue
		}
		if ch == '.' {
			p.pos++
			continue
		}
		if (ch == 'e' || ch == 'E') && p.pos+1 < len(p.input) {
			p.pos++
			if p.input[p.pos] == '+' || p.input[p.pos] == '-' {
				p.pos++
			}
			continue
		}
		break
	}
	if !hasDigit {
		return 0, errors.New("公式格式错误")
	}
	value, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return 0, err
	}
	return sign * value, nil
}

// evaluateFunctions 计算函数值
func (fe *FormulaEvaluator) evaluateFunctions(expression string) string {
	// 处理sum函数
	re := regexp.MustCompile(`sum\(([^()]+)\)`)
	for re.MatchString(expression) {
		matches := re.FindStringSubmatch(expression)
		if len(matches) < 2 {
			break
		}

		var sum float64
		valid := true
		for _, arg := range splitFormulaArgs(matches[1]) {
			value, err := fe.calculate(arg)
			if err != nil {
				valid = false
				break
			}
			sum += value
		}
		if !valid {
			break
		}
		expression = strings.Replace(expression, matches[0], strconv.FormatFloat(sum, 'f', -1, 64), 1)
	}

	// 处理count_ge函数，第一个参数是统一阈值，其余参数是待判断数值
	re = regexp.MustCompile(`count_ge\(([^()]+)\)`)
	for re.MatchString(expression) {
		matches := re.FindStringSubmatch(expression)
		if len(matches) < 2 {
			break
		}

		args := splitFormulaArgs(matches[1])
		if len(args) < 2 {
			break
		}
		threshold, err := fe.calculate(args[0])
		if err != nil {
			break
		}

		count := 0
		for _, arg := range args[1:] {
			value, err := fe.calculate(arg)
			if err != nil {
				continue
			}
			if value >= threshold {
				count++
			}
		}
		expression = strings.Replace(expression, matches[0], strconv.Itoa(count), 1)
	}

	// 处理pow函数
	re = regexp.MustCompile(`pow\(([^,]+),([^()]+)\)`)
	for re.MatchString(expression) {
		matches := re.FindStringSubmatch(expression)
		if len(matches) < 3 {
			break
		}

		// 计算底数
		base, err := fe.calculate(matches[1])
		if err != nil {
			break
		}

		// 计算指数
		exponent, err := fe.calculate(matches[2])
		if err != nil {
			break
		}

		// 计算幂
		result := math.Pow(base, exponent)

		// 替换函数调用为结果
		expression = strings.Replace(expression, matches[0], strconv.FormatFloat(result, 'f', -1, 64), 1)
	}

	// 处理sqrt函数
	re = regexp.MustCompile(`sqrt\(([^()]+)\)`)
	for re.MatchString(expression) {
		matches := re.FindStringSubmatch(expression)
		if len(matches) < 2 {
			break
		}

		// 计算函数参数
		paramValue, err := fe.calculate(matches[1])
		if err != nil {
			break
		}

		// 计算平方根
		result := math.Sqrt(paramValue)

		// 替换函数调用为结果
		expression = strings.Replace(expression, matches[0], strconv.FormatFloat(result, 'f', -1, 64), 1)
	}

	// 处理threshold函数
	re = regexp.MustCompile(`threshold\(([^()]+)\)`)
	for re.MatchString(expression) {
		matches := re.FindStringSubmatch(expression)
		if len(matches) < 2 {
			break
		}

		// 计算函数参数（基因值）
		paramValue, err := fe.calculate(matches[1])
		if err != nil {
			break
		}

		// 获取基因名
		geneName := strings.TrimSpace(matches[1])

		// 检查是否低于阈值
		if threshold, exists := fe.thresholds[geneName]; exists && paramValue < threshold {
			// 低于阈值，返回0
			expression = strings.Replace(expression, matches[0], "0", 1)
		} else {
			// 不低于阈值，返回原值
			expression = strings.Replace(expression, matches[0], strconv.FormatFloat(paramValue, 'f', -1, 64), 1)
		}
	}

	// 处理average_with_threshold函数
	if strings.Contains(expression, "average_with_threshold()") {
		// 计算所有基因的平均值（受阈值影响）
		var sum float64
		var count int
		for geneName, value := range fe.variables {
			if threshold, exists := fe.thresholds[geneName]; exists && value < threshold {
				value = 0
			}
			sum += value
			count++
		}
		var result float64
		if count > 0 {
			result = sum / float64(count)
		}
		expression = strings.Replace(expression, "average_with_threshold()", strconv.FormatFloat(result, 'f', -1, 64), -1)
	}

	// 处理average_without_threshold函数
	if strings.Contains(expression, "average_without_threshold()") {
		// 计算所有基因的平均值（不受阈值影响）
		// 这里需要重新计算，不考虑阈值
		var sum float64
		var count int
		for _, value := range fe.variables {
			// 直接使用原始值，不考虑阈值
			sum += value
			count++
		}
		var result float64
		if count > 0 {
			result = sum / float64(count)
		}
		expression = strings.Replace(expression, "average_without_threshold()", strconv.FormatFloat(result, 'f', -1, 64), -1)
	}

	// 处理sum_with_threshold函数
	if strings.Contains(expression, "sum_with_threshold()") {
		// 计算所有基因的总和（受阈值影响）
		var sum float64
		for geneName, value := range fe.variables {
			if threshold, exists := fe.thresholds[geneName]; exists && value < threshold {
				value = 0
			}
			sum += value
		}
		expression = strings.Replace(expression, "sum_with_threshold()", strconv.FormatFloat(sum, 'f', -1, 64), -1)
	}

	// 处理sum_without_threshold函数
	if strings.Contains(expression, "sum_without_threshold()") {
		// 计算所有基因的总和（不受阈值影响）
		var sum float64
		for _, value := range fe.variables {
			sum += value
		}
		expression = strings.Replace(expression, "sum_without_threshold()", strconv.FormatFloat(sum, 'f', -1, 64), -1)
	}

	return expression
}

func splitFormulaArgs(args string) []string {
	parts := make([]string, 0)
	depth := 0
	start := 0
	for i, r := range args {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(args[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(args[start:]))
	return parts
}

// ExtractVariables 提取公式中的变量
func ExtractVariables(formula string) []string {
	formula = normalizeFormulaText(formula)
	// 匹配变量名（字母开头，包含字母、数字和下划线）
	re := regexp.MustCompile(`\b[a-zA-Z_][a-zA-Z0-9_]*\b`)
	matches := re.FindAllString(formula, -1)

	// 过滤掉函数名
	functionNames := map[string]bool{
		"sqrt":                      true,
		"pow":                       true,
		"sum":                       true,
		"count_ge":                  true,
		"count_ge_threshold":        true,
		"threshold":                 true,
		"average_with_threshold":    true,
		"average_without_threshold": true,
		"sum_with_threshold":        true,
		"sum_without_threshold":     true,
	}

	// 去重
	variableMap := make(map[string]bool)
	for _, match := range matches {
		// 检查是否是函数名
		if !functionNames[match] {
			// 检查是否是数字
			if _, err := strconv.ParseFloat(match, 64); err != nil {
				variableMap[match] = true
			}
		}
	}

	// 转换为切片
	variables := make([]string, 0, len(variableMap))
	for v := range variableMap {
		variables = append(variables, v)
	}

	return variables
}

// EvaluateFormula 直接计算公式结果
func EvaluateFormula(formula string, variables map[string]float64) (float64, error) {
	evaluator := NewFormulaEvaluator(formula)
	evaluator.SetVariables(variables)
	return evaluator.Evaluate()
}

// ValidateFormula 验证公式语法
func ValidateFormula(formula string) error {
	evaluator := NewFormulaEvaluator(formula)
	return evaluator.Validate()
}

// FormulaCalculateRequest 公式计算请求结构
type FormulaCalculateRequest struct {
	Formula    string             `json:"formula" binding:"required"`
	Variables  map[string]float64 `json:"variables"`
	Thresholds map[string]float64 `json:"thresholds"`
}

// HandleFormulaCalculate 处理公式计算请求
func HandleFormulaCalculate(c *app.RequestContext) {
	var req FormulaCalculateRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "请求参数错误",
			Data:    nil,
		})
		return
	}

	// 验证公式
	if err := ValidateFormula(req.Formula); err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	// 创建公式计算器
	evaluator := NewFormulaEvaluator(req.Formula)

	// 设置阈值
	if req.Thresholds != nil {
		evaluator.SetThresholds(req.Thresholds)
	}

	// 设置变量值
	if req.Variables != nil {
		evaluator.SetVariables(req.Variables)
	}

	// 计算结果
	result, err := evaluator.Evaluate()
	if err != nil {
		c.JSON(consts.StatusBadRequest, ApiResponse{
			Code:    400,
			Success: false,
			Message: "公式计算失败: " + err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(consts.StatusOK, ApiResponse{
		Code:    200,
		Success: true,
		Message: "公式计算成功",
		Data:    utils.H{"result": result},
	})
}
