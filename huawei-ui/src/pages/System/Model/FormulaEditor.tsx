import React, { useState } from 'react';
import {
  Modal,
  Button,
  Input,
  Tag,
  Space,
  Row,
  Col,
  message,
  Divider,
  Switch,
} from 'antd';

interface FormulaEditorProps {
  visible?: boolean;
  onCancel: () => void;
  onSave: (formula: string) => void;
  genes: Array<{ id: number; geneSymbol: string }> | null;
  formula: string;
  geneWeights?: Record<string, number>;
  thresholds?: Array<{ geneId: number; threshold: number }>;
}

const normalizeFormulaText = (value: string) => String(value || '').replace(/\s+/g, '');

const getFunctionArgumentLists = (formula: string, functionName: string): string[] => {
  const calls: string[] = [];
  const matcher = new RegExp(`\\b${functionName}\\s*\\(`, 'g');
  let match: RegExpExecArray | null;
  while ((match = matcher.exec(formula)) !== null) {
    const contentStart = matcher.lastIndex;
    let depth = 1;
    let cursor = contentStart;
    for (; cursor < formula.length && depth > 0; cursor += 1) {
      if (formula[cursor] === '(') depth += 1;
      if (formula[cursor] === ')') depth -= 1;
    }
    if (depth === 0) {
      calls.push(formula.slice(contentStart, cursor - 1));
    }
  }
  return calls;
};

const splitTopLevelFormulaArgs = (content: string): string[] => {
  const args: string[] = [];
  let depth = 0;
  let start = 0;
  for (let index = 0; index < content.length; index += 1) {
    if (content[index] === '(') depth += 1;
    if (content[index] === ')') depth -= 1;
    if (content[index] === ',' && depth === 0) {
      args.push(content.slice(start, index).trim());
      start = index + 1;
    }
  }
  args.push(content.slice(start).trim());
  return args;
};

const FormulaEditor: React.FC<FormulaEditorProps> = ({
  visible = true,
  onCancel,
  onSave,
  genes,
  formula: initialFormula,
  geneWeights = {},
  thresholds = [],
}) => {
  const [formula, setFormula] = useState(initialFormula);
  const [error, setError] = useState('');
  const [useThreshold, setUseThreshold] = useState(true);
  const geneList = genes || [];

  const handleInsertGene = (symbol: string) => {
    setFormula(prev => prev + symbol);
  };

  const handleInsertGeneWithThreshold = (symbol: string) => {
    setFormula(prev => prev + `threshold(${symbol})`);
  };

  const handleInsertOperator = (operator: string) => {
    setFormula(prev => prev + operator);
  };

  const handleInsertFunction = (func: string) => {
    // 添加函数并在括号内预留空格，方便用户继续输入
    setFormula(prev => prev + func + '( )');
  };

  const handleInsertAverage = () => {
    if (useThreshold) {
      setFormula(prev => prev + 'average_with_threshold()');
    } else {
      setFormula(prev => prev + 'average_without_threshold()');
    }
  };

  const handleInsertSum = () => {
    if (useThreshold) {
      setFormula(prev => prev + 'sum_with_threshold()');
    } else {
      setFormula(prev => prev + 'sum_without_threshold()');
    }
  };

  const handleInsertContentAtCursor = (content: string, position: number) => {
    // 在指定位置插入内容
    setFormula(prev => prev.substring(0, position) + content + prev.substring(position));
  };

  const validateFormula = (form: string): string => {
    try {
      if (!form) {
        return '公式不能为空';
      }

      // 检查括号匹配（与后端保持一致的实现）
      let bracketCount = 0;
      for (const char of form) {
        if (char === '(') {
          bracketCount++;
        } else if (char === ')') {
          bracketCount--;
          if (bracketCount < 0) {
            return '括号不匹配，请确保所有括号都已闭合';
          }
        }
      }
      if (bracketCount !== 0) {
        return '括号不匹配，请确保所有括号都已闭合';
      }

      // 按括号深度提取函数参数，避免把嵌套 sum(...) 中的逗号算作 pow 的参数。
      const functions = ['sqrt', 'pow', 'sum', 'count_ge', 'count_ge_threshold', 'threshold', 'average_with_threshold', 'average_without_threshold', 'sum_with_threshold', 'sum_without_threshold'];
      for (const func of functions) {
        for (const params of getFunctionArgumentLists(form, func)) {
          const args = splitTopLevelFormulaArgs(params);
          if (func === 'sqrt' && (args.length !== 1 || !args[0])) {
            return 'sqrt函数需要一个参数';
          }
          if (func === 'pow' && (args.length !== 2 || args.some((arg) => !arg))) {
            return 'pow函数需要两个参数，用逗号分隔';
          }
          if (func === 'threshold' && (args.length !== 1 || !args[0])) {
            return 'threshold函数需要一个参数';
          }
        }
      }

      // 检查基本语法，如运算符不能连续出现
      if (/[+\-*/^]{2,}/.test(form)) {
        return '运算符不能连续出现；乘方请使用pow(底数,指数)，普通乘法请使用单个*';
      }

      // 检查公式开头和结尾不能是运算符
      if (/^[+\-*/^]|[+\-*/^]$/.test(form)) {
        return '公式开头和结尾不能是运算符';
      }

      return '';
    } catch (e) {
      return '公式格式错误';
    }
  };

  const handleSave = () => {
    const normalizedFormula = normalizeFormulaText(formula);
    const validationError = validateFormula(normalizedFormula);
    setError(validationError);
    if (!validationError) {
      setFormula(normalizedFormula);
      onSave(normalizedFormula);
    } else {
      message.error(validationError);
    }
  };

  const handleReset = () => {
    setFormula(initialFormula);
    setError('');
  };

  return (
    <Modal
      title="可视化公式构建器"
      open={visible}
      onCancel={onCancel}
      footer={[
        <Button key="reset" onClick={handleReset}>
          重置
        </Button>,
        <Button key="cancel" onClick={onCancel}>
          取消
        </Button>,
        <Button key="save" type="primary" onClick={handleSave}>
          保存公式
        </Button>,
      ]}
      width={800}
    >
      <div style={{ padding: '10px 0' }}>
        {/* 公式编辑区域 - 可编辑 */}
        <div style={{ marginBottom: '20px' }}>
          <h4>当前公式</h4>
          <Input.TextArea
            value={formula}
            onChange={(e) => setFormula(e.target.value)}
            placeholder="直接编辑公式或点击下方按钮构建"
            rows={3}
            style={{ 
              fontFamily: 'monospace', 
              fontSize: '16px',
              minHeight: '80px'
            }}
          />
          {error && (
            <div style={{ color: 'red', marginTop: '5px', fontSize: '12px' }}>
              {error}
            </div>
          )}
        </div>

        <Divider>构建工具</Divider>

        {/* 基因选择区域 */}
        <div style={{ marginBottom: '20px' }}>
          <h4>选中基因</h4>
          <Space wrap>
            {geneList.map(gene => (
              <div key={gene.id} style={{ display: 'flex', alignItems: 'center', marginRight: '10px', marginBottom: '10px' }}>
                <Tag
                  color="blue"
                  onClick={() => handleInsertGene(gene.geneSymbol)}
                  style={{ cursor: 'pointer', padding: '4px 12px', fontSize: '14px', marginRight: '5px' }}
                >
                  {gene.geneSymbol}
                </Tag>
                <Tag
                  color="green"
                  onClick={() => handleInsertGeneWithThreshold(gene.geneSymbol)}
                  style={{ cursor: 'pointer', padding: '4px 8px', fontSize: '12px' }}
                >
                  阈值过滤
                </Tag>
              </div>
            ))}
            {geneList.length === 0 && (
              <Tag color="default">请先在模型设置中选择基因</Tag>
            )}
          </Space>
        </div>

        {/* 运算符区域 */}
        <div style={{ marginBottom: '20px' }}>
          <h4>运算符</h4>
          <Space wrap>
            <Button 
              onClick={() => handleInsertOperator('+')} 
              style={{ width: '60px' }}
            >
              +
            </Button>
            <Button 
              onClick={() => handleInsertOperator('-')} 
              style={{ width: '60px' }}
            >
              -
            </Button>
            <Button 
              onClick={() => handleInsertOperator('*')} 
              style={{ width: '60px' }}
            >
              ×
            </Button>
            <Button 
              onClick={() => handleInsertOperator('/')} 
              style={{ width: '60px' }}
            >
              ÷
            </Button>
            <Button 
              onClick={() => handleInsertOperator('^')} 
              style={{ width: '60px' }}
            >
              ^
            </Button>
            <Button 
              onClick={() => handleInsertOperator('(')} 
              style={{ width: '60px' }}
            >
              (
            </Button>
            <Button 
              onClick={() => handleInsertOperator(')')} 
              style={{ width: '60px' }}
            >
              )
            </Button>
          </Space>
        </div>

        {/* 函数区域 */}
        <div style={{ marginBottom: '20px' }}>
          <h4>函数</h4>
          <Space wrap>
            <Button 
              onClick={() => handleInsertFunction('sqrt')} 
              style={{ width: '100px' }}
            >
              sqrt()
            </Button>
            <Button 
              onClick={() => handleInsertFunction('pow')} 
              style={{ width: '100px' }}
            >
              pow()
            </Button>
            <Button 
              onClick={() => handleInsertFunction('sum')} 
              style={{ width: '100px' }}
            >
              sum()
            </Button>
            <Button 
              onClick={() => handleInsertFunction('count_ge_threshold')} 
              style={{ width: '180px' }}
            >
              count_ge_threshold()
            </Button>
          </Space>
        </div>

        {/* 便捷计算选项 */}
        <div style={{ marginBottom: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
            <h4>便捷计算</h4>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <span style={{ marginRight: '8px', fontSize: '14px' }}>受阈值影响</span>
              <Switch 
                checked={useThreshold} 
                onChange={setUseThreshold} 
              />
            </div>
          </div>
          <Space wrap>
            <Button 
              onClick={handleInsertAverage} 
              style={{ width: '120px' }}
            >
              所有基因平均值
            </Button>
            <Button 
              onClick={handleInsertSum} 
              style={{ width: '120px' }}
            >
              所有基因总和
            </Button>
          </Space>
        </div>
      </div>
    </Modal>
  );
};

export default FormulaEditor;
