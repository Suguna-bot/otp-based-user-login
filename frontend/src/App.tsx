import { FormEvent, useEffect, useRef, useState } from 'react'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

interface User {
  id: number
  email: string
  firstName: string
  lastName: string
}

interface ModalState {
  open: boolean
  email: string
  error: string
  loading: boolean
}

function App() {
  const [activePage, setActivePage] = useState<'checkout' | 'register'>('checkout')
  const [registrationResult, setRegistrationResult] = useState<{ user: User; code: string } | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [recognitionMessage, setRecognitionMessage] = useState('')
  const [modal, setModal] = useState<ModalState>({ open: false, email: '', error: '', loading: false })
  const recognitionTimer = useRef<number | undefined>(undefined)

  useEffect(() => {
    fetch(`${API_URL}/api/auth/me`, { credentials: 'include' })
      .then((response) => response.json())
      .then((data) => {
        if (data.loggedIn) setUser(data.user)
      })
      .catch(() => undefined)
  }, [])

  const checkEmail = (email: string) => {
    setRecognitionMessage('')
    if (recognitionTimer.current) window.clearTimeout(recognitionTimer.current)

    const validEmail = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())
    if (!validEmail || user) return

    recognitionTimer.current = window.setTimeout(async () => {
      try {
        const response = await fetch(`${API_URL}/api/auth/recognize`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email }),
        })
        const data = await response.json()
        if (data.registered) {
          setModal({ open: true, email: email.trim().toLowerCase(), error: '', loading: false })
        }
      } catch {
        setRecognitionMessage('Could not check the email right now.')
      }
    }, 500)
  }

  return (
    <div className="page-shell">
      <header className="topbar">
        <div>
          <div className="brand">OTP Based User Login</div>
          <div className="subtitle">Simple registration and checkout recognition</div>
        </div>
        <nav>
          <button className={activePage === 'checkout' ? 'nav-button active' : 'nav-button'} onClick={() => setActivePage('checkout')}>
            Checkout
          </button>
          <button className={activePage === 'register' ? 'nav-button active' : 'nav-button'} onClick={() => setActivePage('register')}>
            Register
          </button>
        </nav>
      </header>

      <main>
        {activePage === 'register' ? (
          <RegistrationForm
            registrationResult={registrationResult}
            onRegistered={(registeredUser, code) => {
              setUser(null)
              setRegistrationResult({ user: registeredUser, code })
            }}
          />
        ) : (
          <CheckoutForm user={user} onEmailChange={checkEmail} recognitionMessage={recognitionMessage} />
        )}
      </main>

      {modal.open && (
        <OtpModal
          email={modal.email}
          error={modal.error}
          loading={modal.loading}
          onClose={() => setModal((current) => ({ ...current, open: false, error: '' }))}
          onVerified={(loggedInUser) => {
            setUser(loggedInUser)
            setModal((current) => ({ ...current, open: false, error: '' }))
          }}
          setError={(error) => setModal((current) => ({ ...current, error }))}
          setLoading={(loading) => setModal((current) => ({ ...current, loading }))}
        />
      )}
    </div>
  )
}

function RegistrationForm({ onRegistered, registrationResult }: { onRegistered: (user: User, code: string) => void; registrationResult: { user: User; code: string } | null }) {
  const [form, setForm] = useState({ email: '', firstName: '', lastName: '' })
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setError('')
    setMessage('')
    setLoading(true)
    try {
      const response = await fetch(`${API_URL}/api/auth/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      const data = await response.json()
      if (!response.ok) throw new Error(data.error || 'Registration failed')
      setMessage('Account created. Your login code is shown below. Keep it safe because it is required for checkout login.')
      onRegistered(data.user, data.code)
      setForm({ email: '', firstName: '', lastName: '' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="card narrow-card">
      <h1>Create your account</h1>
      <p className="help-text">Enter your details to register. A 6-digit login code will be generated after registration.</p>
      <form onSubmit={submit} className="form-grid">
        <label>
          Email address
          <input type="email" value={form.email} required onChange={(e) => setForm({ ...form, email: e.target.value })} placeholder="you@example.com" />
        </label>
        <label>
          First name
          <input value={form.firstName} required onChange={(e) => setForm({ ...form, firstName: e.target.value })} placeholder="First name" />
        </label>
        <label>
          Last name
          <input value={form.lastName} required onChange={(e) => setForm({ ...form, lastName: e.target.value })} placeholder="Last name" />
        </label>
        {error && <div className="error-box">{error}</div>}
        {message && <div className="success-box">{message}</div>}
        {registrationResult && (
          <div className="code-box">
            <div className="code-label">Your 6-digit login code</div>
            <div className="login-code">{registrationResult.code}</div>
            <div className="code-note">Registered for {registrationResult.user.email}</div>
          </div>
        )}
        <button className="primary-button" disabled={loading}>{loading ? 'Registering...' : 'Register'}</button>
      </form>
    </section>
  )
}

function CheckoutForm({ user, onEmailChange, recognitionMessage }: { user: User | null; onEmailChange: (email: string) => void; recognitionMessage: string }) {
  const [form, setForm] = useState({ email: '', phoneNumber: '', shippingAddress: '' })
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (user) setForm((current) => ({ ...current, email: user.email }))
  }, [user])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setError('')
    setMessage('')
    setLoading(true)
    try {
      const response = await fetch(`${API_URL}/api/checkout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(form),
      })
      const data = await response.json()
      if (!response.ok) throw new Error(data.error || 'Could not save checkout')
      setMessage('Checkout details saved successfully.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not save checkout')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="card">
      <div className="checkout-heading">
        <div>
          <h1>Checkout</h1>
          <p className="help-text">Enter your details below. We will recognize a registered email while you continue filling the form.</p>
        </div>
        {user && <div className="logged-user">Logged in as <strong>{user.firstName} {user.lastName}</strong></div>}
      </div>

      <form onSubmit={submit} className="form-grid">
        <label>
          Email address
          <input
            type="email"
            value={form.email}
            required
            onChange={(e) => {
              const value = e.target.value
              setForm({ ...form, email: value })
              onEmailChange(value)
            }}
            placeholder="you@example.com"
          />
        </label>
        {recognitionMessage && <div className="error-box">{recognitionMessage}</div>}

        <label>
          Phone number
          <input value={form.phoneNumber} required onChange={(e) => setForm({ ...form, phoneNumber: e.target.value })} placeholder="Phone number" />
        </label>

        <label>
          Shipping address
          <textarea value={form.shippingAddress} required onChange={(e) => setForm({ ...form, shippingAddress: e.target.value })} placeholder="Full shipping address" rows={5} />
        </label>

        {error && <div className="error-box">{error}</div>}
        {message && <div className="success-box">{message}</div>}
        <button className="primary-button" disabled={loading}>{loading ? 'Saving...' : 'Submit checkout'}</button>
      </form>
    </section>
  )
}

function OtpModal({ email, error, loading, onClose, onVerified, setError, setLoading }: {
  email: string
  error: string
  loading: boolean
  onClose: () => void
  onVerified: (user: User) => void
  setError: (error: string) => void
  setLoading: (loading: boolean) => void
}) {
  const [code, setCode] = useState('')

  const verify = async (event: FormEvent) => {
    event.preventDefault()
    setError('')
    setLoading(true)
    try {
      const response = await fetch(`${API_URL}/api/auth/verify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ email, code }),
      })
      const data = await response.json()
      if (!response.ok) throw new Error(data.error || 'Invalid code')
      onVerified(data.user)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid code')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-label="Login code">
      <div className="modal-card">
        <h2>Welcome back</h2>
        <p>We found a registered account for <strong>{email}</strong>.</p>
        <p className="help-text">Enter the 6-digit code you received when you registered.</p>
        <form onSubmit={verify} className="form-grid">
          <label>
            Login code
            <input
              inputMode="numeric"
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              placeholder="000000"
              autoFocus
              required
            />
          </label>
          {error && <div className="error-box">{error}</div>}
          <button className="primary-button" disabled={loading || code.length !== 6}>{loading ? 'Checking...' : 'Log in'}</button>
          <button type="button" className="secondary-button" onClick={onClose}>Skip for now</button>
        </form>
      </div>
    </div>
  )
}

export default App
