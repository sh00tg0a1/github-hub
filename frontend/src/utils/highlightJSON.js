// JSON语法高亮函数
const highlightJSON = (json) => {
  // 确保输入是字符串
  const jsonString = typeof json === 'string' ? json : JSON.stringify(json, null, 2)
  
  // 替换特殊字符
  return jsonString
    // 字符串
    .replace(/"([^"]+)"\s*:/g, '<span class="json-key">"$1"</span>:')
    // 字符串值
    .replace(/:\s*"([^"]+)"/g, ': <span class="json-string">"$1"</span>')
    // 数字
    .replace(/:\s*(\d+)/g, ': <span class="json-number">$1</span>')
    // 布尔值
    .replace(/:\s*(true|false)/g, ': <span class="json-boolean">$1</span>')
    // null
    .replace(/:\s*(null)/g, ': <span class="json-null">$1</span>')
}

export default highlightJSON