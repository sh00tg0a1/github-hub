import React, { useState } from 'react'

function MockTest() {
  const [testType, setTestType] = useState('predefined')
  const [eventType, setEventType] = useState('push')
  const [customPayload, setCustomPayload] = useState('')
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState(null)
  const [error, setError] = useState('')

  const handlePredefinedTest = async (e) => {
    e.preventDefault()
    setLoading(true)
    setResult(null)
    setError('')

    try {
      const response = await fetch(`/api/mock/simulate/${eventType}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        }
      })

      const data = await response.json()
      setResult(data)
      if (data.success) {
        // 成功响应，不显示错误
      } else if (data.status === 'skipped') {
        setError('事件被跳过（非main分支或不满足处理条件）')
      } else {
        setError('测试失败：' + (data.message || '未知错误'))
      }
    } catch (error) {
      setError('测试失败：' + error.message)
    } finally {
      setLoading(false)
    }
  }

  const handleCustomTest = async (e) => {
    e.preventDefault()
    setLoading(true)
    setResult(null)
    setError('')

    try {
      // 解析JSON payload
      let parsedPayload
      try {
        parsedPayload = JSON.parse(customPayload)
      } catch (jsonError) {
        setError('JSON格式错误：' + jsonError.message)
        setLoading(false)
        return
      }

      const response = await fetch('/api/custom-test', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          payload: parsedPayload
        })
      })

      const data = await response.json()
      if (data.success) {
        setResult(data)
      } else {
        setError(data.message || '测试失败')
      }
    } catch (error) {
      setError('测试失败：' + error.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="mock-test">
      <div className="card">
        <h2>Mock测试</h2>
        <p>模拟GitHub Webhook事件，测试系统响应</p>

        {/* 测试类型选择 */}
        <div className="form-group">
          <label htmlFor="test-type">测试类型</label>
          <select
            id="test-type"
            value={testType}
            onChange={(e) => setTestType(e.target.value)}
            className="form-control"
          >
            <option value="predefined">预定义测试</option>
            <option value="custom">自定义测试</option>
          </select>
        </div>

        {/* 预定义测试 */}
        {testType === 'predefined' && (
          <form onSubmit={handlePredefinedTest} className="test-section">
            <div className="form-group">
              <label htmlFor="event-type">事件类型</label>
              <select
                id="event-type"
                value={eventType}
                onChange={(e) => setEventType(e.target.value)}
                className="form-control"
                required
              >
                <option value="push">Push (main分支)</option>
                <option value="pull_request.opened">Pull Request (Open)</option>
                <option value="pull_request.synchronize">Pull Request (Synchronize)</option>
              </select>
            </div>

            <p className="help-text">
              点击"提交测试"将使用预定义的 mock 数据模拟 GitHub 事件。支持的事件类型：main 分支 push、PR open、PR synchronize。
            </p>

            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? '测试中...' : '提交测试'}
            </button>

            {error && (
              <div className="test-error" style={{ marginTop: '16px', backgroundColor: '#fee2e2', border: '1px solid #ef4444', borderRadius: '8px', padding: '16px' }}>
                <div style={{ color: '#b91c1c', fontWeight: '600', marginBottom: '8px' }}>测试失败</div>
                <pre style={{ margin: 0, color: '#7f1d1d', fontSize: '14px' }}>{error}</pre>
              </div>
            )}
            {result && (
              <div className="test-result" style={{ marginTop: '16px' }}>
                <h3>测试结果</h3>
                <pre>{JSON.stringify(result, null, 2)}</pre>
              </div>
            )}
          </form>
        )}

        {/* 自定义测试 */}
        {testType === 'custom' && (
          <form onSubmit={handleCustomTest} className="test-section">
            <div className="form-group">
              <label htmlFor="custom-payload">Payload (JSON - 简化格式)</label>
              <textarea
                id="custom-payload"
                value={customPayload}
                onChange={(e) => setCustomPayload(e.target.value)}
                className="form-control"
                rows={15}
                placeholder='示例 Push 事件:
{
  "event_type": "push",
  "repository": "owner/repo",
  "branch": "main",
  "commit_sha": "abc123...",
  "pusher": "username",
  "changed_files": "file1.py,file2.js"
}

示例 PR 事件:
{
  "event_type": "pull_request",
  "repository": "owner/repo",
  "pr_number": 1,
  "pr_action": "opened",
  "pr_title": "Test PR",
  "pr_author": "username",
  "source_branch": "feature/branch",
  "target_branch": "main"
}'
                required
              ></textarea>
            </div>

            <p className="help-text">
              <strong>注意：</strong>自定义测试会检查是否与预定义数据重复。如果参数相同，请使用预定义测试。
            </p>

            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? '测试中...' : '提交测试'}
            </button>

            {error && (
              <div className="test-error" style={{ marginTop: '16px', backgroundColor: '#fee2e2', border: '1px solid #ef4444', borderRadius: '8px', padding: '16px' }}>
                <div style={{ color: '#b91c1c', fontWeight: '600', marginBottom: '8px' }}>测试失败</div>
                <pre style={{ margin: 0, color: '#7f1d1d', fontSize: '14px' }}>{error}</pre>
              </div>
            )}
            {result && (
              <div className="test-result" style={{ marginTop: '16px' }}>
                <h3>测试结果</h3>
                <pre>{JSON.stringify(result, null, 2)}</pre>
              </div>
            )}
          </form>
        )}
      </div>
    </div>
  )
}

export default MockTest