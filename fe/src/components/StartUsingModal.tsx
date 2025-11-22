import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Service } from '../types/api'

interface StartUsingModalProps {
  service: Service
  onClose: () => void
  onSubscribe?: (serviceId: number) => Promise<{ apiKey: string; endpoint: string }>
}

export function StartUsingModal({ service, onClose, onSubscribe }: StartUsingModalProps) {
  const { t } = useTranslation()
  const [step, setStep] = useState<'confirm' | 'credentials'>('confirm')
  const [isLoading, setIsLoading] = useState(false)
  const [credentials, setCredentials] = useState<{ apiKey: string; endpoint: string } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copiedField, setCopiedField] = useState<string | null>(null)

  const estimatedMonthlyCost = 100 // Example: 100K tokens
  const estimatedCost = (estimatedMonthlyCost * service.pricePer1KTokens).toFixed(2)

  const handleSubscribe = async () => {
    if (!onSubscribe) {
      // Mock credentials for UI development
      setCredentials({
        apiKey: 'tok_' + Math.random().toString(36).substring(2, 15),
        endpoint: `https://gateway.tokligence.ai/v1/${service.modelFamily}`,
      })
      setStep('credentials')
      return
    }

    setIsLoading(true)
    setError(null)

    try {
      const creds = await onSubscribe(service.id)
      setCredentials(creds)
      setStep('credentials')
    } catch (err) {
      setError((err as Error).message || 'Failed to subscribe')
    } finally {
      setIsLoading(false)
    }
  }

  const copyToClipboard = async (text: string, field: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedField(field)
      setTimeout(() => setCopiedField(null), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-2xl rounded-xl bg-white shadow-xl">
        {/* Header */}
        <div className="border-b border-slate-200 px-6 py-4">
          <div className="flex items-start justify-between">
            <div>
              <h2 className="text-xl font-bold text-slate-900">
                {step === 'confirm' ? 'Subscribe to Service' : 'API Credentials'}
              </h2>
              <p className="mt-1 text-sm text-slate-500">{service.name}</p>
            </div>
            <button
              onClick={onClose}
              className="ml-4 rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            >
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="p-6">
          {step === 'confirm' && (
            <div className="space-y-6">
              {/* Service Details Recap */}
              <section className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <h3 className="text-sm font-semibold text-slate-900">Service Details</h3>
                <div className="mt-3 space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-slate-600">Model:</span>
                    <span className="font-medium text-slate-900">{service.modelFamily?.toUpperCase()}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Provider:</span>
                    <span className="font-medium text-slate-900">{service.providerName || 'Unknown'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Price per 1K tokens:</span>
                    <span className="font-medium text-slate-900">${service.pricePer1KTokens.toFixed(4)}</span>
                  </div>
                </div>
              </section>

              {/* Trial Offer */}
              {service.trialTokens && service.trialTokens > 0 && (
                <section className="rounded-lg border border-blue-200 bg-blue-50 p-4">
                  <div className="flex items-start gap-3">
                    <div className="text-2xl">🎁</div>
                    <div>
                      <h3 className="text-sm font-semibold text-blue-900">Free Trial Available</h3>
                      <p className="mt-1 text-sm text-blue-700">
                        Get {service.trialTokens.toLocaleString()} free tokens to try this service
                      </p>
                    </div>
                  </div>
                </section>
              )}

              {/* Cost Estimator */}
              <section className="rounded-lg border border-slate-200 bg-white p-4">
                <h3 className="text-sm font-semibold text-slate-900">Estimated Monthly Cost</h3>
                <p className="mt-1 text-xs text-slate-500">Based on {estimatedMonthlyCost}K tokens usage</p>
                <div className="mt-3 flex items-baseline gap-2">
                  <span className="text-3xl font-bold text-slate-900">${estimatedCost}</span>
                  <span className="text-sm text-slate-500">/ month</span>
                </div>
                <p className="mt-2 text-xs text-slate-600">
                  💡 Actual cost depends on your usage. You'll only pay for tokens consumed.
                </p>
              </section>

              {/* Billing Info */}
              <section className="rounded-lg border border-amber-200 bg-amber-50 p-4">
                <div className="flex items-start gap-3">
                  <div className="text-xl">💳</div>
                  <div>
                    <h3 className="text-sm font-semibold text-amber-900">Pay-as-you-go Billing</h3>
                    <p className="mt-1 text-sm text-amber-700">
                      No upfront fees or subscriptions. You're only charged for the tokens you actually use.
                    </p>
                  </div>
                </div>
              </section>

              {/* Error */}
              {error && (
                <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">{error}</div>
              )}
            </div>
          )}

          {step === 'credentials' && credentials && (
            <div className="space-y-6">
              {/* Success Message */}
              <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4">
                <div className="flex items-start gap-3">
                  <div className="text-2xl">✅</div>
                  <div>
                    <h3 className="text-sm font-semibold text-emerald-900">Successfully Subscribed!</h3>
                    <p className="mt-1 text-sm text-emerald-700">
                      You can now use this service with the credentials below.
                    </p>
                  </div>
                </div>
              </div>

              {/* API Credentials */}
              <section className="space-y-4">
                <div>
                  <label className="block text-sm font-semibold text-slate-900">API Endpoint</label>
                  <div className="mt-2 flex items-center gap-2">
                    <input
                      type="text"
                      value={credentials.endpoint}
                      readOnly
                      className="flex-1 rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 font-mono text-sm"
                    />
                    <button
                      onClick={() => copyToClipboard(credentials.endpoint, 'endpoint')}
                      className="rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium hover:bg-slate-50"
                    >
                      {copiedField === 'endpoint' ? '✓ Copied' : 'Copy'}
                    </button>
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">API Key</label>
                  <p className="mt-1 text-xs text-amber-600">⚠️ Save this key now. It won't be shown again.</p>
                  <div className="mt-2 flex items-center gap-2">
                    <input
                      type="text"
                      value={credentials.apiKey}
                      readOnly
                      className="flex-1 rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 font-mono text-sm"
                    />
                    <button
                      onClick={() => copyToClipboard(credentials.apiKey, 'apiKey')}
                      className="rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium hover:bg-slate-50"
                    >
                      {copiedField === 'apiKey' ? '✓ Copied' : 'Copy'}
                    </button>
                  </div>
                </div>
              </section>

              {/* Quick Start Examples */}
              <section className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <h3 className="text-sm font-semibold text-slate-900">Quick Start</h3>
                <div className="mt-3 space-y-3">
                  {/* cURL Example */}
                  <div>
                    <div className="text-xs font-medium text-slate-600">cURL</div>
                    <pre className="mt-1 overflow-x-auto rounded bg-slate-900 p-3 text-xs text-slate-100">
{`curl ${credentials.endpoint}/chat/completions \\
  -H "Authorization: Bearer ${credentials.apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${service.modelFamily}",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'`}
                    </pre>
                  </div>

                  {/* Python Example */}
                  <div>
                    <div className="text-xs font-medium text-slate-600">Python</div>
                    <pre className="mt-1 overflow-x-auto rounded bg-slate-900 p-3 text-xs text-slate-100">
{`import openai

client = openai.OpenAI(
    api_key="${credentials.apiKey}",
    base_url="${credentials.endpoint}"
)

response = client.chat.completions.create(
    model="${service.modelFamily}",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)`}
                    </pre>
                  </div>
                </div>
              </section>

              {/* Next Steps */}
              <section className="rounded-lg border border-blue-200 bg-blue-50 p-4">
                <h3 className="text-sm font-semibold text-blue-900">Next Steps</h3>
                <ul className="mt-2 space-y-1 text-sm text-blue-700">
                  <li>• View usage and billing in your Dashboard</li>
                  <li>• Monitor API calls and token consumption</li>
                  <li>• Configure rate limits and alerts</li>
                </ul>
              </section>
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="border-t border-slate-200 px-6 py-4">
          <div className="flex items-center justify-end gap-3">
            {step === 'confirm' && (
              <>
                <button
                  onClick={onClose}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
                >
                  Cancel
                </button>
                <button
                  onClick={handleSubscribe}
                  disabled={isLoading}
                  className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
                >
                  {isLoading ? 'Subscribing...' : 'Confirm Subscription'}
                </button>
              </>
            )}
            {step === 'credentials' && (
              <button
                onClick={onClose}
                className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
              >
                Done
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
