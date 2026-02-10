import React, { useState, useEffect } from 'react'

function SystemStatus() {
  const [status, setStatus] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetchStatus()
  }, [])

  const fetchStatus = async () => {
    try {
      setLoading(true)
      const response = await fetch('/api/status')
      const data = await response.json()
      if (data.success) {
        setStatus(data.data)
      } else {
        setError(data.message || '加载系统状态失败')
      }
    } catch (error) {
      setError('加载系统状态失败：' + error.message)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return <div className="loading">加载中...</div>
  }

  return (
    <div className="system-status">
      <div className="card">
        <h2>系统状态</h2>
        <p>监控系统运行状态和服务健康情况</p>

        {error && (
          <div className="message message-error">{error}</div>
        )}

        {status && (
          <div className="status-container">
            <div className="status-item">
              <h3>服务状态</h3>
              <div className={`status-indicator ${status.service_status === 'healthy' ? 'healthy' : 'unhealthy'}`}>
                {status.service_status === 'healthy' ? '正常' : '异常'}
              </div>
            </div>

            <div className="status-item">
              <h3>数据库状态</h3>
              <div className={`status-indicator ${status.database_status === 'connected' ? 'healthy' : 'unhealthy'}`}>
                {status.database_status === 'connected' ? '已连接' : '未连接'}
              </div>
            </div>

            <div className="status-item">
              <h3>系统信息</h3>
              <div className="status-details">
                <p>版本: {status.version || '1.0.0'}</p>
                <p>运行时间: {status.uptime || 'N/A'}</p>
                <p>事件处理数: {status.total_events || 0}</p>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default SystemStatus