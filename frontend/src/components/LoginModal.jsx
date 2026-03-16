import React, { useState } from 'react'

function LoginModal({ onLogin, onClose }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e) => {
    e.preventDefault()

    if (!username || !password) {
      setError('用户名和密码不能为空')
      return
    }

    setLoading(true)
    setError('')

    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username, password }),
        credentials: 'include'
      })

      const data = await response.json()

      if (data.success) {
        onLogin(data.username)
      } else {
        setError(data.error || '登录失败')
      }
    } catch (error) {
      setError('登录失败：' + error.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="login-modal" onClick={(e) => e.stopPropagation()}>
        {/* 关闭按钮 */}
        <button className="login-close-btn" onClick={onClose}>
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>

        {/* 头部 */}
        <div className="login-header">
          <div className="login-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
              <path d="M12 15v3m-9 3h18M9 12h9m-9-4h9m-7-4a9 9 0 1118 0 9 9 0 01-18 0z" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <h2>管理员登录</h2>
          <p>请输入管理员凭证以继续</p>
        </div>

        {/* 表单 */}
        <form onSubmit={handleSubmit} className="login-form">
          {error && (
            <div className="login-error">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <path d="M12 8v4M12 16h.01" />
              </svg>
              {error}
            </div>
          )}

          <div className="login-field">
            <label htmlFor="username">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2" strokeLinecap="round" strokeLinejoin="round" />
                <circle cx="12" cy="7" r="4" />
              </svg>
              用户名
            </label>
            <input
              type="text"
              id="username"
              className="login-input"
              placeholder="请输入用户名"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
            />
          </div>

          <div className="login-field">
            <label htmlFor="password">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                <path d="M7 11V7a5 5 0 0110 0v4" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              密码
            </label>
            <input
              type="password"
              id="password"
              className="login-input"
              placeholder="请输入密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>

          <button type="submit" className="login-submit-btn" disabled={loading}>
            {loading ? (
              <>
                <span className="login-spinner"></span>
                登录中...
              </>
            ) : (
              <>
                登录
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M14 5l7 7m0 0l-7 7m7-7H3" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </>
            )}
          </button>
        </form>
      </div>

      {/* 登录模态框专属样式 - 明亮主题 */}
      <style>{`
        .login-modal {
          background: var(--bg-secondary);
          border-radius: var(--radius-2xl);
          padding: 0;
          width: 100%;
          max-width: 420px;
          box-shadow: var(--shadow-xl);
          border: 1px solid var(--border-color);
          position: relative;
          animation: slideUp 0.3s ease;
          overflow: hidden;
        }

        .login-close-btn {
          position: absolute;
          top: 20px;
          right: 20px;
          background: transparent;
          border: none;
          color: var(--text-muted);
          cursor: pointer;
          padding: 8px;
          border-radius: var(--radius-sm);
          transition: all var(--transition-fast);
          display: flex;
          align-items: center;
          justify-content: center;
        }

        .login-close-btn:hover {
          color: var(--text-primary);
          background: var(--bg-hover);
        }

        .login-header {
          background: linear-gradient(135deg, var(--primary-color), var(--secondary-color));
          padding: 40px 32px 32px;
          text-align: center;
          position: relative;
          overflow: hidden;
        }

        .login-header::before {
          content: '';
          position: absolute;
          top: -50%;
          left: -50%;
          width: 200%;
          height: 200%;
          background: radial-gradient(circle, rgba(255,255,255,0.15) 0%, transparent 70%);
          animation: float 6s ease-in-out infinite;
        }

        @keyframes float {
          0%, 100% {
            transform: translate(0, 0) rotate(0deg);
          }
          50% {
            transform: translate(30px, -30px) rotate(180deg);
          }
        }

        .login-icon {
          width: 64px;
          height: 64px;
          background: rgba(255,255,255,0.25);
          border-radius: 50%;
          display: flex;
          align-items: center;
          justify-content: center;
          margin: 0 auto 16px;
          color: white;
          position: relative;
          z-index: 1;
          box-shadow: 0 4px 12px rgba(0,0,0,0.1);
        }

        .login-icon svg {
          width: 32px;
          height: 32px;
        }

        .login-header h2 {
          color: white;
          font-size: 1.5rem;
          font-weight: 600;
          margin: 0 0 8px 0;
          position: relative;
          z-index: 1;
        }

        .login-header p {
          color: rgba(255,255,255,0.9);
          font-size: 0.9rem;
          margin: 0;
          position: relative;
          z-index: 1;
        }

        .login-form {
          padding: 32px;
        }

        .login-error {
          background: var(--danger-light);
          color: #991b1b;
          padding: 12px 16px;
          border-radius: var(--radius-md);
          margin-bottom: 20px;
          display: flex;
          align-items: center;
          gap: 10px;
          font-size: 0.9rem;
          border: 1px solid rgba(239, 68, 68, 0.2);
          animation: shake 0.4s ease;
        }

        .login-error svg {
          flex-shrink: 0;
        }

        @keyframes shake {
          0%, 100% { transform: translateX(0); }
          25% { transform: translateX(-4px); }
          75% { transform: translateX(4px); }
        }

        .login-field {
          margin-bottom: 20px;
        }

        .login-field label {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 0.85rem;
          font-weight: 600;
          color: var(--text-primary);
          margin-bottom: 8px;
        }

        .login-field label svg {
          flex-shrink: 0;
          color: var(--primary-color);
        }

        .login-input {
          width: 100%;
          padding: 14px 16px;
          background: var(--bg-tertiary);
          border: 1px solid var(--border-color);
          border-radius: var(--radius-md);
          color: var(--text-primary);
          font-size: 0.95rem;
          transition: all var(--transition-normal);
        }

        .login-input:focus {
          outline: none;
          border-color: var(--primary-color);
          box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
          background: var(--bg-secondary);
        }

        .login-input::placeholder {
          color: var(--text-muted);
        }

        .login-submit-btn {
          width: 100%;
          padding: 14px 24px;
          background: linear-gradient(135deg, var(--primary-color), var(--primary-hover));
          color: white;
          border: none;
          border-radius: var(--radius-md);
          font-size: 1rem;
          font-weight: 600;
          cursor: pointer;
          transition: all var(--transition-normal);
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 8px;
          box-shadow: var(--shadow-colorful);
          margin-top: 8px;
        }

        .login-submit-btn:hover:not(:disabled) {
          transform: translateY(-2px);
          box-shadow: 0 8px 25px rgba(99, 102, 241, 0.35);
        }

        .login-submit-btn:disabled {
          opacity: 0.7;
          cursor: not-allowed;
        }

        .login-spinner {
          width: 16px;
          height: 16px;
          border: 2px solid rgba(255,255,255,0.3);
          border-top-color: white;
          border-radius: 50%;
          animation: spin 1s linear infinite;
        }

        @keyframes spin {
          to { transform: rotate(360deg); }
        }

        .login-footer {
          margin-top: 24px;
          padding-top: 20px;
          border-top: 1px solid var(--border-color);
          text-align: center;
        }

        .login-footer p {
          color: var(--text-muted);
          font-size: 0.8rem;
        }

        @media (max-width: 480px) {
          .login-modal {
            max-width: calc(100vw - 32px);
            margin: 16px;
          }

          .login-header {
            padding: 32px 24px 24px;
          }

          .login-form {
            padding: 24px;
          }
        }
      `}</style>
    </div>
  )
}

export default LoginModal
