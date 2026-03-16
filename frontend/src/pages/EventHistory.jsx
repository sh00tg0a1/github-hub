import React, { useState, useEffect } from 'react'
import highlightJSON from '../utils/highlightJSON'

function EventHistory({ isLoggedIn }) {
  const [events, setEvents] = useState([])
  const [initialLoading, setInitialLoading] = useState(true)
  const [pageLoading, setPageLoading] = useState(false)
  const [error, setError] = useState('')
  const [selectedEvent, setSelectedEvent] = useState(null)
  const [showDetails, setShowDetails] = useState(false)
  const [detailsLoading, setDetailsLoading] = useState(false)

  // 分页状态
  const [pagination, setPagination] = useState({
    page: 1,
    page_size: 20,
    total: 0,
    total_pages: 0
  })

  useEffect(() => {
    fetchEvents()
  }, [pagination.page])

  const fetchEvents = async (isInitial = false) => {
    try {
      if (isInitial) {
        setInitialLoading(true)
      } else {
        setPageLoading(true)
      }
      const params = new URLSearchParams({
        page: pagination.page.toString(),
        page_size: pagination.page_size.toString()
      })
      const response = await fetch(`/api/events?${params}`)
      const data = await response.json()
      if (data.success) {
        setEvents(data.data || [])
        setPagination(data.pagination || pagination)
      } else {
        setError(data.message || '加载事件失败')
      }
    } catch (error) {
      setError('加载事件失败：' + error.message)
    } finally {
      if (isInitial) {
        setInitialLoading(false)
      } else {
        setPageLoading(false)
      }
    }
  }

  // 首次加载
  useEffect(() => {
    fetchEvents(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handlePageChange = (newPage) => {
    if (newPage >= 1 && newPage <= pagination.total_pages) {
      setPagination({
        ...pagination,
        page: newPage
      })
    }
  }

  const handlePageSizeChange = (newPageSize) => {
    setPagination({
      ...pagination,
      page_size: newPageSize,
      page: 1  // 重置到第一页
    })
  }

  const handleDeleteEvent = async (eventId) => {
    // 使用原生 confirm 确保先弹框确认，用户确认后才执行删除
    const confirmed = window.confirm('确定要删除这个事件吗？此操作不可恢复。')
    if (!confirmed) {
      return
    }

    try {
      const response = await fetch(`/api/events/${eventId}`, {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json'
        },
        credentials: 'include'
      })

      const data = await response.json()
      if (data.success) {
        fetchEvents()
      } else {
        setError(data.message || '删除失败')
      }
    } catch (error) {
      setError('删除失败：' + error.message)
    }
  }

  const handleClearDatabase = async () => {
    // 使用原生 confirm 确保先弹框确认，用户确认后才执行删除
    const confirmed = window.confirm('危险操作！确定要清空整个数据库吗？所有事件记录将被永久删除，此操作不可恢复。')
    if (!confirmed) {
      return
    }

    try {
      const response = await fetch('/api/events', {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json'
        },
        credentials: 'include'
      })

      const data = await response.json()
      if (data.success) {
        fetchEvents()
      } else {
        setError(data.message || '清空数据库失败')
      }
    } catch (error) {
      setError('清空数据库失败：' + error.message)
    }
  }

  const handleShowDetails = async (eventId) => {
    try {
      setDetailsLoading(true)
      const response = await fetch(`/api/events/${eventId}`)
      const data = await response.json()
      if (data.success) {
        setSelectedEvent(data.data)
        setShowDetails(true)
      } else {
        setError(data.message || '获取事件详情失败')
      }
    } catch (error) {
      setError('获取事件详情失败：' + error.message)
    } finally {
      setDetailsLoading(false)
    }
  }

  const handleCloseDetails = () => {
    setShowDetails(false)
    setSelectedEvent(null)
  }

  const getStatusClass = (status) => {
    const normalizedStatus = status.toLowerCase()
    switch (normalizedStatus) {
      case 'pending':
        return 'status-pending'
      case 'completed':
        return 'status-completed'
      case 'failed':
        return 'status-failed'
      case 'skipped':
        return 'status-skipped'
      default:
        return 'status-processing'
    }
  }

  const getStatusText = (status) => {
    const normalizedStatus = status.toLowerCase()
    switch (normalizedStatus) {
      case 'pending':
        return '待处理'
      case 'completed':
        return '已完成'
      case 'failed':
        return '失败'
      case 'skipped':
        return '跳过'
      default:
        return '处理中'
    }
  }

  const getCheckTypeText = (checkType) => {
    const normalizedType = checkType.toLowerCase()
    const typeMap = {
      'compilation': '编译检查',
      'code_lint': '代码规范检查',
      'security_scan': '安全扫描',
      'unit_test': '单元测试',
      'deployment': '部署',
      'api_test': 'API测试',
      'module_e2e': '模块端到端测试',
      'agent_e2e': 'Agent端到端测试',
      'ai_e2e': 'AI端到端测试'
    }
    return typeMap[normalizedType] || checkType
  }

  const getCheckStatusClass = (status) => {
    const normalizedStatus = status.toLowerCase()
    switch (normalizedStatus) {
      case 'pending':
        return 'status-pending'
      case 'running':
        return 'status-processing'
      case 'passed':
        return 'status-completed'
      case 'failed':
        return 'status-failed'
      case 'skipped':
        return 'status-skipped'
      case 'cancelled':
        return 'status-skipped'
      default:
        return ''
    }
  }

  const getCheckStatusText = (status) => {
    const normalizedStatus = status.toLowerCase()
    switch (normalizedStatus) {
      case 'pending':
        return '未开始'
      case 'running':
        return '运行中'
      case 'passed':
        return '通过'
      case 'failed':
        return '失败'
      case 'skipped':
        return '跳过'
      case 'cancelled':
        return '取消'
      default:
        return status
    }
  }

  // 生成页码数组
  const getPageNumbers = () => {
    const pages = []
    const { page, total_pages } = pagination

    // 总是显示第一页
    pages.push(1)

    // 添加省略号和当前页附近的页码
    if (total_pages <= 7) {
      for (let i = 2; i <= total_pages; i++) {
        pages.push(i)
      }
    } else {
      if (page > 4) {
        pages.push('...')
      }

      let start = Math.max(2, page - 2)
      let end = Math.min(page + 2, total_pages - 1)

      if (page <= 3) {
        start = 2
        end = 5
      } else if (page >= total_pages - 2) {
        start = total_pages - 4
        end = total_pages - 1
      }

      for (let i = start; i <= end; i++) {
        pages.push(i)
      }

      if (page < total_pages - 3) {
        pages.push('...')
      }

      // 总是显示最后一页
      if (total_pages > 1) {
        pages.push(total_pages)
      }
    }

    return pages
  }

  if (initialLoading) {
    return <div className="loading">加载中...</div>
  }

  return (
    <div>
      <div className="event-history">
        <div className="card">
          <h2>GitHub 事件</h2>
          <p>查看和管理所有 GitHub 事件记录</p>

          {isLoggedIn && (
            <div className="action-buttons">
              <button
                className="btn btn-danger"
                onClick={handleClearDatabase}
              >
                一键清理数据库
              </button>
            </div>
          )}

          {error && (
            <div className="message message-error">{error}</div>
          )}

          {pageLoading && (
            <div className="message message-info" style={{ marginBottom: '16px' }}>
              加载中...
            </div>
          )}

          {/* 表格容器 - 可滚动区域 */}
          <div className="table-container">
            {events.length > 0 ? (
              <table className="table">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>事件类型</th>
                    <th>仓库</th>
                    <th>分支</th>
                    <th>状态</th>
                    <th>时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map(event => (
                    <tr key={event.id}>
                      <td title={event.id}>{event.id}</td>
                      <td title={event.event_type}>{event.event_type}</td>
                      <td title={event.repository}>{event.repository}</td>
                      <td title={event.branch}>{event.branch}</td>
                      <td title={getStatusText(event.event_status)}>
                        <span className={`status ${getStatusClass(event.event_status)}`}>
                          {getStatusText(event.event_status)}
                        </span>
                      </td>
                      <td title={event.created_at}>{event.created_at}</td>
                      <td>
                        <button
                          className="btn btn-info"
                          onClick={() => handleShowDetails(event.id)}
                        >
                          详情
                        </button>
                        {isLoggedIn && (
                          <button
                            className="btn btn-danger"
                            onClick={() => handleDeleteEvent(event.id)}
                            style={{ marginLeft: '8px' }}
                          >
                            删除
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="empty-state">
                <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <p>暂无事件记录</p>
                <p className="empty-state-hint">当接收到 GitHub Webhook 事件后，此处将显示事件列表</p>
              </div>
            )}
          </div>

          {/* 分页控件 - 固定在卡片底部 */}
          {pagination.total_pages > 1 && (
            <div className="pagination" style={{
              marginTop: '20px',
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              gap: '10px',
              flexWrap: 'wrap'
            }}>
              {/* 每页显示数量选择 */}
              <div className="page-size-selector" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span>每页显示:</span>
                <select
                  value={pagination.page_size}
                  onChange={(e) => handlePageSizeChange(parseInt(e.target.value))}
                  style={{ padding: '5px 10px', borderRadius: '4px', border: '1px solid #ddd' }}
                >
                  <option value="10">10</option>
                  <option value="20">20</option>
                  <option value="50">50</option>
                  <option value="100">100</option>
                </select>
              </div>

              {/* 分页按钮 */}
              <div className="pagination-buttons" style={{ display: 'flex', gap: '5px' }}>
                <button
                  className="btn"
                  onClick={() => handlePageChange(1)}
                  disabled={pagination.page === 1}
                  style={{ opacity: pagination.page === 1 ? 0.5 : 1 }}
                >
                  首页
                </button>
                <button
                  className="btn"
                  onClick={() => handlePageChange(pagination.page - 1)}
                  disabled={pagination.page === 1}
                  style={{ opacity: pagination.page === 1 ? 0.5 : 1 }}
                >
                  上一页
                </button>

                {getPageNumbers().map((pageNum, index) => (
                  pageNum === '...' ? (
                    <span key={`ellipsis-${index}`} style={{ padding: '5px' }}>...</span>
                  ) : (
                    <button
                      key={pageNum}
                      className="btn"
                      onClick={() => handlePageChange(pageNum)}
                      disabled={pageNum === pagination.page}
                      style={{
                        padding: '5px 12px',
                        backgroundColor: pageNum === pagination.page ? '#007bff' : '#f8f9fa',
                        color: pageNum === pagination.page ? '#fff' : '#333',
                        border: '1px solid #ddd'
                      }}
                    >
                      {pageNum}
                    </button>
                  )
                ))}

                <button
                  className="btn"
                  onClick={() => handlePageChange(pagination.page + 1)}
                  disabled={pagination.page === pagination.total_pages}
                  style={{ opacity: pagination.page === pagination.total_pages ? 0.5 : 1 }}
                >
                  下一页
                </button>
                <button
                  className="btn"
                  onClick={() => handlePageChange(pagination.total_pages)}
                  disabled={pagination.page === pagination.total_pages}
                  style={{ opacity: pagination.page === pagination.total_pages ? 0.5 : 1 }}
                >
                  末页
                </button>
              </div>

              {/* 分页信息 */}
              <div className="pagination-info" style={{ marginLeft: '20px', color: '#666', fontSize: '14px' }}>
                共 {pagination.total} 条记录 / {pagination.total_pages} 页
                <br />
                当前第 {pagination.page} 页
              </div>
            </div>
          )}
        </div>
      </div>

      {showDetails && selectedEvent && (
        <div className="modal-overlay">
          <div className="modal-content" style={{ maxWidth: '800px', maxHeight: '80vh', overflowY: 'auto' }}>
            <div className="modal-header">
              <h3>事件详情</h3>
              <button 
                className="btn btn-close"
                onClick={handleCloseDetails}
                style={{ background: 'none', border: 'none', fontSize: '24px', cursor: 'pointer' }}
              >
                ×
              </button>
            </div>
            <div className="modal-body">
              {detailsLoading ? (
                <div className="loading">加载中...</div>
              ) : (
                <>
                  <div className="event-details">
                    <h4>基本信息</h4>
                    <div className="detail-grid">
                      <div className="detail-item">
                        <span className="detail-label">ID:</span>
                        <span className="detail-value">{selectedEvent.id}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">事件类型:</span>
                        <span className="detail-value">{selectedEvent.event_type}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">仓库:</span>
                        <span className="detail-value">{selectedEvent.repository}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">分支:</span>
                        <span className="detail-value">{selectedEvent.branch}</span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">状态:</span>
                        <span className={`detail-value status ${getStatusClass(selectedEvent.event_status)}`}>
                          {getStatusText(selectedEvent.event_status)}
                        </span>
                      </div>
                      <div className="detail-item">
                        <span className="detail-label">创建时间:</span>
                        <span className="detail-value">{selectedEvent.created_at}</span>
                      </div>
                      {selectedEvent.target_branch && (
                        <div className="detail-item">
                          <span className="detail-label">目标分支:</span>
                          <span className="detail-value">{selectedEvent.target_branch}</span>
                        </div>
                      )}
                      {selectedEvent.pr_number && (
                        <div className="detail-item">
                          <span className="detail-label">PR编号:</span>
                          <span className="detail-value">{selectedEvent.pr_number}</span>
                        </div>
                      )}
                    </div>
                  </div>

                  {selectedEvent.quality_checks && selectedEvent.quality_checks.length > 0 && (
                    <div className="quality-checks" style={{ marginTop: '20px' }}>
                      <h4>质量检查项</h4>
                      <div className="quality-pipeline">
                        {/* 基础CI流水线 */}
                        <div className="quality-stage">
                          <h5>基础CI流水线</h5>
                          <div className="stage-checks">
                            {selectedEvent.quality_checks
                              .filter(check => check.stage === 'basic_ci')
                              .map(check => (
                                <div key={check.id} className="quality-check">
                                  <div className="check-header">
                                    <span className="check-type">{getCheckTypeText(check.check_type)}</span>
                                    <span className={`status ${getCheckStatusClass(check.check_status)}`}>
                                      {getCheckStatusText(check.check_status)}
                                    </span>
                                  </div>
                                  {check.error_message && (
                                    <div className="check-error">{check.error_message}</div>
                                  )}
                                </div>
                              ))
                            }
                          </div>
                        </div>

                        {/* 部署阶段 */}
                        <div className="quality-stage" style={{ marginTop: '20px' }}>
                          <h5>测试环境部署</h5>
                          <div className="stage-checks">
                            {selectedEvent.quality_checks
                              .filter(check => check.stage === 'deployment')
                              .map(check => (
                                <div key={check.id} className="quality-check">
                                  <div className="check-header">
                                    <span className="check-type">{getCheckTypeText(check.check_type)}</span>
                                    <span className={`status ${getCheckStatusClass(check.check_status)}`}>
                                      {getCheckStatusText(check.check_status)}
                                    </span>
                                  </div>
                                  {check.error_message && (
                                    <div className="check-error">{check.error_message}</div>
                                  )}
                                </div>
                              ))
                            }
                          </div>
                        </div>

                        {/* 专项测试流水线 */}
                        <div className="quality-stage" style={{ marginTop: '20px' }}>
                          <h5>专项测试流水线</h5>
                          <div className="stage-checks">
                            {selectedEvent.quality_checks
                              .filter(check => check.stage === 'specialized_tests')
                              .map(check => (
                                <div key={check.id} className="quality-check">
                                  <div className="check-header">
                                    <span className="check-type">{getCheckTypeText(check.check_type)}</span>
                                    <span className={`status ${getCheckStatusClass(check.check_status)}`}>
                                      {getCheckStatusText(check.check_status)}
                                    </span>
                                  </div>
                                  {check.error_message && (
                                    <div className="check-error">{check.error_message}</div>
                                  )}
                                </div>
                              ))
                            }
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Payload信息 */}
                  {selectedEvent.payload && Object.keys(selectedEvent.payload).length > 0 && (
                    <div className="payload-info" style={{ marginTop: '20px' }}>
                      <h4>Payload信息</h4>
                      <div className="payload-content" style={{ 
                        backgroundColor: '#f5f5f5', 
                        padding: '15px', 
                        borderRadius: '4px', 
                        maxHeight: '400px', 
                        overflow: 'auto',
                        fontFamily: 'monospace',
                        fontSize: '12px'
                      }}>
                        <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }} 
                             dangerouslySetInnerHTML={{ __html: highlightJSON(selectedEvent.payload) }} />
                      </div>
                    </div>
                  )}
                </>
              )}
            </div>
            <div className="modal-footer">
              <button 
                className="btn btn-primary"
                onClick={handleCloseDetails}
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default EventHistory