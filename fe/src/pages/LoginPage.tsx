import { useState } from 'react'
import { requestAuthLogin, requestAuthVerify, isUnauthorized } from '../services/api'

interface LoginPageProps {
  onSuccess: () => void
}

export function LoginPage({ onSuccess }: LoginPageProps) {
  const [email, setEmail] = useState('')
  const [challengeId, setChallengeId] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [enableProvider, setEnableProvider] = useState(false)
  const [status, setStatus] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string>('')
  const [codeHint, setCodeHint] = useState<string>('')
  const [expiresAt, setExpiresAt] = useState<string>('')

  const requestCode = async () => {
    setLoading(true)
    setError('')
    setStatus('Sending verification code…')
    try {
      const resp = await requestAuthLogin(email.trim())
      if ('token' in resp) {
        setStatus('Signed in as gateway administrator.')
        setChallengeId(null)
        setCodeHint('')
        setExpiresAt('')
        onSuccess()
        return
      }
      setChallengeId(resp.challenge_id)
      setCodeHint(resp.code)
      setExpiresAt(resp.expires_at)
      setStatus('Verification code sent. Check your email (code also shown below for local testing).')
    } catch (err) {
      if (isUnauthorized(err)) {
        setError('Email verification is currently disabled.')
      } else {
        setError((err as Error).message ?? 'Failed to send code')
      }
      setChallengeId(null)
      setCodeHint('')
    } finally {
      setLoading(false)
    }
  }

  const verify = async () => {
    if (!challengeId) {
      setError('Please request a verification code first.')
      return
    }
    setLoading(true)
    setError('')
    setStatus('Verifying code…')
    try {
      await requestAuthVerify({
        challenge_id: challengeId,
        code: code.trim(),
        display_name: displayName.trim() || undefined,
        enable_provider: enableProvider,
      })
      setStatus('Verification successful!')
      onSuccess()
    } catch (err) {
      setError((err as Error).message ?? 'Verification failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
      <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-8 shadow-lg">
        <h1 className="text-2xl font-semibold text-slate-900">Sign in to Tokligence Gateway</h1>
        <p className="mt-2 text-sm text-slate-500">
          Enter your email to receive a verification code. During development the response includes the code for
          convenience.
        </p>

        <label className="mt-6 block text-sm font-medium text-slate-700">
          Email
          <input
            type="email"
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-slate-900 focus:border-slate-500 focus:outline-none"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
          />
        </label>

        <button
          type="button"
          className="mt-4 w-full rounded-lg bg-slate-900 px-3 py-2 text-sm font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
          onClick={requestCode}
          disabled={loading || !email}
        >
          Send code
        </button>

        {challengeId && (
          <div className="mt-6 rounded-lg bg-slate-50 p-4 text-sm text-slate-600">
            <p className="font-medium text-slate-800">Challenge ID</p>
            <p className="mt-1 break-all text-xs text-slate-500">{challengeId}</p>
            {codeHint && (
              <p className="mt-2 text-xs text-emerald-600">Dev code: {codeHint}</p>
            )}
            {expiresAt && <p className="mt-1 text-xs text-slate-500">Expires at: {new Date(expiresAt).toLocaleString()}</p>}
          </div>
        )}

        <label className="mt-6 block text-sm font-medium text-slate-700">
          Verification code
          <input
            type="text"
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-slate-900 focus:border-slate-500 focus:outline-none"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="6-digit code"
          />
        </label>

        <label className="mt-4 block text-sm font-medium text-slate-700">
          Display name (optional)
          <input
            type="text"
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-slate-900 focus:border-slate-500 focus:outline-none"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Tokligence Gateway"
          />
        </label>

        <label className="mt-4 flex items-center gap-2 text-sm text-slate-600">
          <input type="checkbox" checked={enableProvider} onChange={(e) => setEnableProvider(e.target.checked)} />
          Enable provider role for this session
        </label>

        <button
          type="button"
          className="mt-4 w-full rounded-lg bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
          onClick={verify}
          disabled={loading || !code || !challengeId}
        >
          Verify & continue
        </button>

        {status && <p className="mt-4 text-xs text-slate-500">{status}</p>}
        {error && <p className="mt-2 text-xs text-rose-600">{error}</p>}
      </div>
    </div>
  )
}
