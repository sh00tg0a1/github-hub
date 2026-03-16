import React, { useState, useEffect } from 'react'
import LoginModal from './components/LoginModal'
import MockTest from './pages/MockTest'
import EventHistory from './pages/EventHistory'
import SystemStatus from './pages/SystemStatus'

function App() {
  const [currentTab, setCurrentTab] = useState('event-history')
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [username, setUsername] = useState('')
  const [showLoginModal, setShowLoginModal] = useState(false)

  // 检查登录状态
  useEffect(() => {
    checkLoginStatus()
  }, [])

  const checkLoginStatus = async () => {
    try {
      const response = await fetch('/api/check-login', {
        credentials: 'include'
      })
      const data = await response.json()
      if (data.is_logged_in) {
        setIsLoggedIn(true)
        setUsername(data.username)
      }
    } catch (error) {
      console.error('检查登录状态失败:', error)
    }
  }

  const handleLogin = (username) => {
    setIsLoggedIn(true)
    setUsername(username)
    setShowLoginModal(false)
  }

  const handleLogout = async () => {
    try {
      const response = await fetch('/api/logout', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        credentials: 'include'
      })
      const data = await response.json()
      if (data.success) {
        setIsLoggedIn(false)
        setUsername('')
      }
    } catch (error) {
      console.error('退出登录失败:', error)
    }
  }

  const renderTabContent = () => {
    switch (currentTab) {
      case 'mock-test':
        return <MockTest />
      case 'event-history':
        return <EventHistory isLoggedIn={isLoggedIn} />
      case 'system-status':
        return <SystemStatus />
      default:
        return <EventHistory isLoggedIn={isLoggedIn} />
    }
  }

  return (
    <div className="app">
      {/* 头部导航 */}
      <header>
        <div className="container header-content">
          <div className="logo">Dule Quality Engine</div>
          <nav>
            <div className="nav-links">
              <button
                  className={`nav-btn ${currentTab === 'event-history' ? 'active' : ''}`}
                  onClick={() => setCurrentTab('event-history')}
                >
                  GitHub 事件
                </button>
              <button
                className={`nav-link ${currentTab === 'mock-test' ? 'active' : ''}`}
                onClick={() => setCurrentTab('mock-test')}
              >
                Mock测试
              </button>
              <button
                className={`nav-link ${currentTab === 'system-status' ? 'active' : ''}`}
                onClick={() => setCurrentTab('system-status')}
              >
                系统状态
              </button>
            </div>
            <div className="admin-controls">
              {isLoggedIn ? (
                <button className="admin-btn" onClick={handleLogout}>
                  登出
                </button>
              ) : (
                <button className="admin-btn" onClick={() => setShowLoginModal(true)}>
                  管理
                </button>
              )}
            </div>
          </nav>
        </div>
      </header>

      {/* 主内容区 */}
      <main className="content-container">
        {renderTabContent()}
      </main>

      {/* 页脚 */}
      <footer>
        <div className="container footer-content">
          <div className="footer-info">
            <p>&copy; 2026 Dule Quality Engine. All rights reserved.</p>
          </div>
          <div className="footer-links">
            <a href="#">关于</a>
            <a href="#">文档</a>
            <a href="#">支持</a>
          </div>
        </div>
      </footer>

      {/* 登录弹窗 */}
      {showLoginModal && (
        <LoginModal onLogin={handleLogin} onClose={() => setShowLoginModal(false)} />
      )}
    </div>
  )
}

export default App