import { useState } from 'react'
import type { Service } from '../types/api'

interface PublishServiceModalProps {
  onClose: () => void
  onPublish?: (service: Partial<Service>) => Promise<void>
}

type Step = 1 | 2 | 3 | 4 | 5 | 6

export function PublishServiceModal({ onClose, onPublish }: PublishServiceModalProps) {
  // const { t } = useTranslation() // TODO: Add translations
  const [currentStep, setCurrentStep] = useState<Step>(1)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Form state
  const [formData, setFormData] = useState<Partial<Service>>({
    name: '',
    description: '',
    modelFamily: 'gpt',
    baseModel: '',
    pricePer1KTokens: 0,
    trialTokens: 0,
    contextWindow: 8000,
    maxOutputTokens: 4000,
    features: {
      functionCalling: false,
      vision: false,
      streaming: true,
      jsonMode: false,
    },
    geographic: {
      country: '',
      region: '',
      city: '',
      dataCenters: [],
    },
    compliance: [],
    metrics: {
      latencyP50: 0,
      latencyP95: 0,
      latencyP99: 0,
      throughputTps: 0,
    },
    availability: {
      schedule: '24/7',
      timezone: 'UTC',
    },
  })

  const updateFormData = (updates: Partial<Service>) => {
    setFormData((prev) => ({ ...prev, ...updates }))
  }

  const handleNext = () => {
    if (currentStep < 6) {
      setCurrentStep((prev) => (prev + 1) as Step)
    }
  }

  const handlePrev = () => {
    if (currentStep > 1) {
      setCurrentStep((prev) => (prev - 1) as Step)
    }
  }

  const handlePublish = async () => {
    setIsSubmitting(true)
    setError(null)

    try {
      if (onPublish) {
        await onPublish(formData)
      } else {
        // Mock publish
        await new Promise((resolve) => setTimeout(resolve, 1000))
        console.log('Publishing service:', formData)
      }
      onClose()
    } catch (err) {
      setError((err as Error).message || 'Failed to publish service')
    } finally {
      setIsSubmitting(false)
    }
  }

  const steps = [
    { number: 1, name: 'Basic Info', description: 'Service details' },
    { number: 2, name: 'Pricing', description: 'Set your rates' },
    { number: 3, name: 'Technical', description: 'Specs & features' },
    { number: 4, name: 'Infrastructure', description: 'Location & compliance' },
    { number: 5, name: 'Performance', description: 'Metrics & SLAs' },
    { number: 6, name: 'Review', description: 'Confirm & publish' },
  ]

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-xl bg-white shadow-xl">
        {/* Header */}
        <div className="sticky top-0 z-10 border-b border-slate-200 bg-white px-6 py-4">
          <div className="flex items-start justify-between">
            <div>
              <h2 className="text-xl font-bold text-slate-900">Publish New Service</h2>
              <p className="mt-1 text-sm text-slate-500">Step {currentStep} of 6: {steps[currentStep - 1].description}</p>
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

          {/* Progress Bar */}
          <div className="mt-4 flex items-center gap-2">
            {steps.map((step) => (
              <div key={step.number} className="flex-1">
                <div
                  className={`h-2 rounded-full ${
                    step.number <= currentStep ? 'bg-slate-900' : 'bg-slate-200'
                  }`}
                />
                <div className="mt-1 text-xs text-slate-500">{step.name}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Content */}
        <div className="p-6">
          {/* Step 1: Basic Info */}
          {currentStep === 1 && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-semibold text-slate-900">Service Name*</label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => updateFormData({ name: e.target.value })}
                  placeholder="e.g., Fast GPT-4 API"
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                />
              </div>

              <div>
                <label className="block text-sm font-semibold text-slate-900">Description*</label>
                <textarea
                  value={formData.description}
                  onChange={(e) => updateFormData({ description: e.target.value })}
                  placeholder="Describe your service, its unique features, and benefits..."
                  rows={4}
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-sm font-semibold text-slate-900">Model Family*</label>
                  <select
                    value={formData.modelFamily}
                    onChange={(e) => updateFormData({ modelFamily: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  >
                    <option value="gpt">GPT (OpenAI Compatible)</option>
                    <option value="claude">Claude (Anthropic Compatible)</option>
                    <option value="llama">Llama (Meta)</option>
                    <option value="gemini">Gemini (Google)</option>
                    <option value="mistral">Mistral</option>
                    <option value="other">Other</option>
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">Base Model</label>
                  <input
                    type="text"
                    value={formData.baseModel}
                    onChange={(e) => updateFormData({ baseModel: e.target.value })}
                    placeholder="e.g., gpt-4-turbo, llama-3-70b"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>
              </div>
            </div>
          )}

          {/* Step 2: Pricing */}
          {currentStep === 2 && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-semibold text-slate-900">Price per 1K Tokens (USD)*</label>
                <div className="mt-1 flex items-center gap-2">
                  <span className="text-slate-500">$</span>
                  <input
                    type="number"
                    step="0.0001"
                    min="0"
                    value={formData.pricePer1KTokens}
                    onChange={(e) => updateFormData({ pricePer1KTokens: parseFloat(e.target.value) })}
                    className="flex-1 rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>
                <p className="mt-1 text-xs text-slate-500">
                  Example: $0.0030 for GPT-4, $0.0001 for smaller models
                </p>
              </div>

              <div>
                <label className="block text-sm font-semibold text-slate-900">Free Trial Tokens (Optional)</label>
                <input
                  type="number"
                  min="0"
                  step="1000"
                  value={formData.trialTokens}
                  onChange={(e) => updateFormData({ trialTokens: parseInt(e.target.value) })}
                  placeholder="e.g., 10000"
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                />
                <p className="mt-1 text-xs text-slate-500">
                  Offer free tokens to help users try your service
                </p>
              </div>

              <div className="rounded-lg border border-blue-200 bg-blue-50 p-4">
                <h4 className="text-sm font-semibold text-blue-900">Pricing Preview</h4>
                <div className="mt-2 space-y-1 text-sm text-blue-700">
                  <div className="flex justify-between">
                    <span>100K tokens:</span>
                    <span className="font-semibold">${(formData.pricePer1KTokens! * 100).toFixed(2)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>1M tokens:</span>
                    <span className="font-semibold">${(formData.pricePer1KTokens! * 1000).toFixed(2)}</span>
                  </div>
                  {formData.trialTokens! > 0 && (
                    <div className="mt-2 border-t border-blue-300 pt-2">
                      <span>Free trial value: </span>
                      <span className="font-semibold">
                        ${((formData.trialTokens! / 1000) * formData.pricePer1KTokens!).toFixed(2)}
                      </span>
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Step 3: Technical Specs */}
          {currentStep === 3 && (
            <div className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-sm font-semibold text-slate-900">Context Window (tokens)*</label>
                  <input
                    type="number"
                    min="0"
                    step="1000"
                    value={formData.contextWindow}
                    onChange={(e) => updateFormData({ contextWindow: parseInt(e.target.value) })}
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                  <p className="mt-1 text-xs text-slate-500">e.g., 8000, 32000, 128000</p>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">Max Output Tokens*</label>
                  <input
                    type="number"
                    min="0"
                    step="100"
                    value={formData.maxOutputTokens}
                    onChange={(e) => updateFormData({ maxOutputTokens: parseInt(e.target.value) })}
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-semibold text-slate-900">Supported Features</label>
                <div className="mt-2 space-y-2">
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={formData.features?.functionCalling}
                      onChange={(e) =>
                        updateFormData({
                          features: { ...formData.features!, functionCalling: e.target.checked },
                        })
                      }
                      className="rounded"
                    />
                    <span className="text-sm text-slate-700">Function Calling / Tool Use</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={formData.features?.vision}
                      onChange={(e) =>
                        updateFormData({
                          features: { ...formData.features!, vision: e.target.checked },
                        })
                      }
                      className="rounded"
                    />
                    <span className="text-sm text-slate-700">Vision / Image Input</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={formData.features?.streaming}
                      onChange={(e) =>
                        updateFormData({
                          features: { ...formData.features!, streaming: e.target.checked },
                        })
                      }
                      className="rounded"
                    />
                    <span className="text-sm text-slate-700">Streaming Responses</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={formData.features?.jsonMode}
                      onChange={(e) =>
                        updateFormData({
                          features: { ...formData.features!, jsonMode: e.target.checked },
                        })
                      }
                      className="rounded"
                    />
                    <span className="text-sm text-slate-700">JSON Mode / Structured Output</span>
                  </label>
                </div>
              </div>
            </div>
          )}

          {/* Step 4: Infrastructure */}
          {currentStep === 4 && (
            <div className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-3">
                <div>
                  <label className="block text-sm font-semibold text-slate-900">Country*</label>
                  <input
                    type="text"
                    value={formData.geographic?.country}
                    onChange={(e) =>
                      updateFormData({
                        geographic: { ...formData.geographic!, country: e.target.value },
                      })
                    }
                    placeholder="e.g., USA, China, Germany"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">Region</label>
                  <input
                    type="text"
                    value={formData.geographic?.region}
                    onChange={(e) =>
                      updateFormData({
                        geographic: { ...formData.geographic!, region: e.target.value },
                      })
                    }
                    placeholder="e.g., US-West, EU-Central"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">City</label>
                  <input
                    type="text"
                    value={formData.geographic?.city}
                    onChange={(e) =>
                      updateFormData({
                        geographic: { ...formData.geographic!, city: e.target.value },
                      })
                    }
                    placeholder="e.g., San Francisco"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-semibold text-slate-900">Data Center Locations</label>
                <input
                  type="text"
                  placeholder="Comma-separated: us-west-1, eu-central-1"
                  onChange={(e) =>
                    updateFormData({
                      geographic: {
                        ...formData.geographic!,
                        dataCenters: e.target.value.split(',').map((s) => s.trim()).filter(Boolean),
                      },
                    })
                  }
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                />
              </div>

              <div>
                <label className="block text-sm font-semibold text-slate-900">Compliance Certifications</label>
                <div className="mt-2 space-y-2">
                  {['GDPR', 'SOC2', 'HIPAA', 'ISO27001', 'PCI-DSS'].map((cert) => (
                    <label key={cert} className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={formData.compliance?.includes(cert)}
                        onChange={(e) => {
                          const current = formData.compliance || []
                          const updated = e.target.checked
                            ? [...current, cert]
                            : current.filter((c) => c !== cert)
                          updateFormData({ compliance: updated })
                        }}
                        className="rounded"
                      />
                      <span className="text-sm text-slate-700">{cert}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* Step 5: Performance */}
          {currentStep === 5 && (
            <div className="space-y-4">
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-4">
                <p className="text-sm text-amber-800">
                  💡 These metrics help customers choose the right service. Provide realistic estimates based on your infrastructure.
                </p>
              </div>

              <div className="grid gap-4 sm:grid-cols-3">
                <div>
                  <label className="block text-sm font-semibold text-slate-900">Latency p50 (ms)</label>
                  <input
                    type="number"
                    min="0"
                    value={formData.metrics?.latencyP50}
                    onChange={(e) =>
                      updateFormData({
                        metrics: { ...formData.metrics!, latencyP50: parseInt(e.target.value) },
                      })
                    }
                    placeholder="e.g., 50"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">Latency p95 (ms)</label>
                  <input
                    type="number"
                    min="0"
                    value={formData.metrics?.latencyP95}
                    onChange={(e) =>
                      updateFormData({
                        metrics: { ...formData.metrics!, latencyP95: parseInt(e.target.value) },
                      })
                    }
                    placeholder="e.g., 150"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">Latency p99 (ms)</label>
                  <input
                    type="number"
                    min="0"
                    value={formData.metrics?.latencyP99}
                    onChange={(e) =>
                      updateFormData({
                        metrics: { ...formData.metrics!, latencyP99: parseInt(e.target.value) },
                      })
                    }
                    placeholder="e.g., 300"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-semibold text-slate-900">Throughput (tokens/second)</label>
                <input
                  type="number"
                  min="0"
                  value={formData.metrics?.throughputTps}
                  onChange={(e) =>
                    updateFormData({
                      metrics: { ...formData.metrics!, throughputTps: parseInt(e.target.value) },
                    })
                  }
                  placeholder="e.g., 1000"
                  className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-sm font-semibold text-slate-900">Availability Schedule</label>
                  <select
                    value={formData.availability?.schedule}
                    onChange={(e) =>
                      updateFormData({
                        availability: {
                          ...formData.availability!,
                          schedule: e.target.value as '24/7' | 'business_hours' | 'custom',
                        },
                      })
                    }
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  >
                    <option value="24/7">24/7 Available</option>
                    <option value="business_hours">Business Hours Only</option>
                    <option value="custom">Custom Schedule</option>
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-slate-900">Timezone</label>
                  <input
                    type="text"
                    value={formData.availability?.timezone}
                    onChange={(e) =>
                      updateFormData({
                        availability: { ...formData.availability!, timezone: e.target.value },
                      })
                    }
                    placeholder="e.g., UTC, America/New_York"
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2"
                  />
                </div>
              </div>
            </div>
          )}

          {/* Step 6: Review */}
          {currentStep === 6 && (
            <div className="space-y-6">
              <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <h3 className="text-base font-semibold text-slate-900">Service Overview</h3>
                <dl className="mt-3 space-y-2 text-sm">
                  <div className="flex justify-between">
                    <dt className="text-slate-600">Name:</dt>
                    <dd className="font-medium text-slate-900">{formData.name || 'Not set'}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-600">Model:</dt>
                    <dd className="font-medium text-slate-900">{formData.modelFamily?.toUpperCase()}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-600">Price:</dt>
                    <dd className="font-medium text-slate-900">${formData.pricePer1KTokens?.toFixed(4)} / 1K tokens</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-600">Context Window:</dt>
                    <dd className="font-medium text-slate-900">{formData.contextWindow?.toLocaleString()} tokens</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-600">Location:</dt>
                    <dd className="font-medium text-slate-900">
                      {formData.geographic?.city}, {formData.geographic?.country}
                    </dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-600">Availability:</dt>
                    <dd className="font-medium text-slate-900">{formData.availability?.schedule}</dd>
                  </div>
                </dl>
              </div>

              <div className="rounded-lg border border-blue-200 bg-blue-50 p-4">
                <h4 className="text-sm font-semibold text-blue-900">What happens next?</h4>
                <ul className="mt-2 space-y-1 text-sm text-blue-700">
                  <li>• Your service will be published to the marketplace immediately</li>
                  <li>• Customers can discover and subscribe to your service</li>
                  <li>• You'll start earning revenue from token usage</li>
                  <li>• Track performance in your Provider Dashboard</li>
                </ul>
              </div>

              {error && (
                <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">{error}</div>
              )}
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="sticky bottom-0 border-t border-slate-200 bg-white px-6 py-4">
          <div className="flex items-center justify-between">
            <button
              onClick={currentStep === 1 ? onClose : handlePrev}
              className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              {currentStep === 1 ? 'Cancel' : 'Previous'}
            </button>

            {currentStep < 6 ? (
              <button
                onClick={handleNext}
                disabled={!formData.name || !formData.description}
                className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
              >
                Next
              </button>
            ) : (
              <button
                onClick={handlePublish}
                disabled={isSubmitting}
                className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
              >
                {isSubmitting ? 'Publishing...' : 'Publish Service'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
