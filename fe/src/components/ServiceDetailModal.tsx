import { useTranslation } from 'react-i18next'
import type { Service } from '../types/api'

interface ServiceDetailModalProps {
  service: Service
  onClose: () => void
  onStartUsing: (service: Service) => void
}

export function ServiceDetailModal({ service, onClose, onStartUsing }: ServiceDetailModalProps) {
  const { t } = useTranslation()

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-xl bg-white shadow-xl">
        {/* Header */}
        <div className="sticky top-0 z-10 border-b border-slate-200 bg-white px-6 py-4">
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <h2 className="text-xl font-bold text-slate-900">{service.name}</h2>
              <p className="mt-1 text-sm text-slate-500">{service.modelFamily?.toUpperCase()}</p>
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
        <div className="space-y-6 p-6">
          {/* Provider Info */}
          <section>
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Provider</h3>
            <div className="mt-2 flex items-center gap-2">
              <span className="text-base font-medium text-slate-900">{service.providerName || 'Unknown Provider'}</span>
              {service.providerVerified && (
                <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700">
                  ✓ Verified
                </span>
              )}
            </div>
          </section>

          {/* Description */}
          <section>
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Description</h3>
            <p className="mt-2 text-sm text-slate-700">{service.description || 'No description available.'}</p>
          </section>

          {/* Pricing */}
          <section>
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Pricing</h3>
            <div className="mt-2 grid gap-4 sm:grid-cols-2">
              {service.inputPricePer1MTokens !== undefined && service.outputPricePer1MTokens !== undefined ? (
                <>
                  <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                    <div className="text-xs text-slate-500 mb-1">Input Price</div>
                    <div className="text-2xl font-bold text-slate-900">${service.inputPricePer1MTokens.toFixed(2)}</div>
                    <div className="text-xs text-slate-500">per 1M tokens</div>
                  </div>
                  <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                    <div className="text-xs text-slate-500 mb-1">Output Price</div>
                    <div className="text-2xl font-bold text-slate-900">${service.outputPricePer1MTokens.toFixed(2)}</div>
                    <div className="text-xs text-slate-500">per 1M tokens</div>
                  </div>
                </>
              ) : service.pricePer1KTokens === 0 ? (
                <div className="rounded-lg border border-green-200 bg-green-50 p-4">
                  <div className="text-2xl font-bold text-green-900">FREE</div>
                  <div className="text-xs text-green-600">Open-source / Self-hosted</div>
                </div>
              ) : (
                <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                  <div className="text-2xl font-bold text-slate-900">${service.pricePer1KTokens.toFixed(4)}</div>
                  <div className="text-xs text-slate-500">per 1K tokens</div>
                </div>
              )}
              {service.trialTokens && service.trialTokens > 0 && (
                <div className="rounded-lg border border-blue-200 bg-blue-50 p-4">
                  <div className="text-2xl font-bold text-blue-900">{service.trialTokens.toLocaleString()}</div>
                  <div className="text-xs text-blue-600">free trial tokens</div>
                </div>
              )}
            </div>
          </section>

          {/* Deployment Type */}
          {service.deploymentType && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Deployment</h3>
              <div className="mt-2">
                {service.deploymentType === 'self-hosted' && (
                  <div className="rounded-lg border border-purple-200 bg-purple-50 p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <span className="text-base font-semibold text-purple-900">Self-Hosted Only</span>
                      <span className="rounded-full bg-purple-200 px-2 py-0.5 text-xs font-medium text-purple-800">
                        Requires Your Infrastructure
                      </span>
                    </div>
                    {service.selfHostedConfig && (
                      <div className="mt-3 space-y-2 text-sm text-purple-800">
                        {service.selfHostedConfig.minimumSpecs && (
                          <div>
                            <span className="font-medium">Minimum Specs:</span> {service.selfHostedConfig.minimumSpecs}
                          </div>
                        )}
                        {service.selfHostedConfig.dockerImage && (
                          <div>
                            <span className="font-medium">Docker Image:</span>{' '}
                            <code className="rounded bg-purple-100 px-1.5 py-0.5 text-xs">{service.selfHostedConfig.dockerImage}</code>
                          </div>
                        )}
                        {service.selfHostedConfig.setupComplexity && (
                          <div>
                            <span className="font-medium">Setup Complexity:</span>{' '}
                            <span className="capitalize">{service.selfHostedConfig.setupComplexity}</span>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
                {service.deploymentType === 'cloud' && (
                  <div className="rounded-lg border border-blue-200 bg-blue-50 p-4">
                    <span className="text-base font-semibold text-blue-900">Cloud-Hosted</span>
                    <p className="mt-1 text-sm text-blue-700">Fully managed service - no infrastructure setup required</p>
                  </div>
                )}
                {service.deploymentType === 'both' && (
                  <div className="rounded-lg border border-indigo-200 bg-indigo-50 p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <span className="text-base font-semibold text-indigo-900">Hybrid Deployment</span>
                      <span className="rounded-full bg-indigo-200 px-2 py-0.5 text-xs font-medium text-indigo-800">
                        Cloud OR Self-Hosted
                      </span>
                    </div>
                    <p className="text-sm text-indigo-700">Available as both managed cloud service and self-hosted option</p>
                    {service.selfHostedConfig?.minimumSpecs && (
                      <div className="mt-2 text-xs text-indigo-600">
                        Self-hosting requires: {service.selfHostedConfig.minimumSpecs}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </section>
          )}

          {/* Modalities */}
          {(service.inputModalities || service.outputModalities) && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Modalities</h3>
              <div className="mt-2 grid gap-3 sm:grid-cols-2">
                {service.inputModalities && service.inputModalities.length > 0 && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500 mb-2">Input</div>
                    <div className="flex flex-wrap gap-1.5">
                      {service.inputModalities.map((modality) => (
                        <span key={modality} className="rounded-full bg-blue-100 px-2.5 py-1 text-xs font-medium text-blue-700 capitalize">
                          {modality}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
                {service.outputModalities && service.outputModalities.length > 0 && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500 mb-2">Output</div>
                    <div className="flex flex-wrap gap-1.5">
                      {service.outputModalities.map((modality) => (
                        <span key={modality} className="rounded-full bg-green-100 px-2.5 py-1 text-xs font-medium text-green-700 capitalize">
                          {modality}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </section>
          )}

          {/* Use Cases */}
          {service.useCases && service.useCases.length > 0 && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Recommended Use Cases</h3>
              <div className="mt-2 flex flex-wrap gap-2">
                {service.useCases.map((useCase) => (
                  <span key={useCase} className="rounded-lg bg-amber-100 px-3 py-1.5 text-sm font-medium text-amber-800 capitalize">
                    {useCase.replace('-', ' ')}
                  </span>
                ))}
              </div>
            </section>
          )}

          {/* Technical Specifications */}
          <section>
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Technical Specifications</h3>
            <div className="mt-2 grid gap-3 sm:grid-cols-2">
              <div className="rounded-lg border border-slate-200 bg-white p-3">
                <div className="text-xs text-slate-500">Context Window</div>
                <div className="mt-1 text-sm font-semibold text-slate-900">
                  {service.contextWindow ? `${(service.contextWindow / 1000).toFixed(0)}K tokens` : 'N/A'}
                </div>
              </div>
              <div className="rounded-lg border border-slate-200 bg-white p-3">
                <div className="text-xs text-slate-500">Max Output Tokens</div>
                <div className="mt-1 text-sm font-semibold text-slate-900">
                  {service.maxOutputTokens ? service.maxOutputTokens.toLocaleString() : 'N/A'}
                </div>
              </div>
            </div>

            {/* Features */}
            {service.features && (
              <div className="mt-3 flex flex-wrap gap-2">
                {service.features.functionCalling && (
                  <span className="rounded-full bg-purple-100 px-3 py-1 text-xs font-medium text-purple-700">
                    Function Calling
                  </span>
                )}
                {service.features.vision && (
                  <span className="rounded-full bg-pink-100 px-3 py-1 text-xs font-medium text-pink-700">
                    Vision
                  </span>
                )}
                {service.features.streaming && (
                  <span className="rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700">
                    Streaming
                  </span>
                )}
                {service.features.jsonMode && (
                  <span className="rounded-full bg-green-100 px-3 py-1 text-xs font-medium text-green-700">
                    JSON Mode
                  </span>
                )}
              </div>
            )}
          </section>

          {/* Geographic Information */}
          {service.geographic && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Geographic Location</h3>
              <div className="mt-2 grid gap-3 sm:grid-cols-3">
                {service.geographic.country && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Country</div>
                    <div className="mt-1 text-sm font-semibold text-slate-900">{service.geographic.country}</div>
                  </div>
                )}
                {service.geographic.region && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Region</div>
                    <div className="mt-1 text-sm font-semibold text-slate-900">{service.geographic.region}</div>
                  </div>
                )}
                {service.geographic.city && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">City</div>
                    <div className="mt-1 text-sm font-semibold text-slate-900">{service.geographic.city}</div>
                  </div>
                )}
              </div>
              {service.geographic.dataCenters && service.geographic.dataCenters.length > 0 && (
                <div className="mt-2 text-xs text-slate-600">
                  Data Centers: {service.geographic.dataCenters.join(', ')}
                </div>
              )}
            </section>
          )}

          {/* Performance Metrics */}
          {service.metrics && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Performance Metrics</h3>
              <div className="mt-2 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {/* Uptime */}
                {service.metrics.uptime30d !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Uptime (30 days)</div>
                    <div className="mt-1 flex items-baseline gap-1">
                      <div className="text-lg font-bold text-emerald-600">{service.metrics.uptime30d.toFixed(2)}%</div>
                    </div>
                  </div>
                )}

                {/* Latency p50 */}
                {service.metrics.latencyP50 !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Latency (p50)</div>
                    <div className="mt-1 text-lg font-bold text-slate-900">{service.metrics.latencyP50}ms</div>
                  </div>
                )}

                {/* Latency p95 */}
                {service.metrics.latencyP95 !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Latency (p95)</div>
                    <div className="mt-1 text-lg font-bold text-slate-900">{service.metrics.latencyP95}ms</div>
                  </div>
                )}

                {/* Latency p99 */}
                {service.metrics.latencyP99 !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Latency (p99)</div>
                    <div className="mt-1 text-lg font-bold text-slate-900">{service.metrics.latencyP99}ms</div>
                  </div>
                )}

                {/* Throughput */}
                {service.metrics.throughputTps !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Throughput</div>
                    <div className="mt-1 text-lg font-bold text-slate-900">
                      {service.metrics.throughputTps.toLocaleString()} tokens/s
                    </div>
                  </div>
                )}
              </div>
            </section>
          )}

          {/* Availability */}
          {service.availability && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Availability</h3>
              <div className="mt-2 rounded-lg border border-slate-200 bg-white p-3">
                <div className="flex items-center gap-2">
                  {service.availability.schedule === '24/7' && (
                    <span className="rounded-full bg-emerald-100 px-3 py-1 text-xs font-medium text-emerald-700">
                      ⏰ 24/7 Available
                    </span>
                  )}
                  {service.availability.schedule === 'business_hours' && (
                    <span className="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-700">
                      ⏰ Business Hours Only
                    </span>
                  )}
                  {service.availability.timezone && (
                    <span className="text-xs text-slate-500">Timezone: {service.availability.timezone}</span>
                  )}
                </div>
              </div>
            </section>
          )}

          {/* Reviews & Ratings */}
          {service.rating !== undefined && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Reviews & Ratings</h3>
              <div className="mt-2 flex items-center gap-4">
                <div className="flex items-baseline gap-1">
                  <span className="text-3xl font-bold text-slate-900">{service.rating.toFixed(1)}</span>
                  <span className="text-lg text-slate-400">/ 5</span>
                </div>
                <div className="flex items-center gap-1">
                  {[...Array(5)].map((_, i) => (
                    <svg
                      key={i}
                      className={`h-5 w-5 ${i < Math.floor(service.rating || 0) ? 'text-yellow-400' : 'text-slate-300'}`}
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                    </svg>
                  ))}
                </div>
                {service.reviewCount !== undefined && (
                  <span className="text-sm text-slate-500">({service.reviewCount.toLocaleString()} reviews)</span>
                )}
              </div>
            </section>
          )}

          {/* Usage Statistics */}
          {service.usageStats && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Usage Statistics</h3>
              <div className="mt-2 grid gap-3 sm:grid-cols-3">
                {service.usageStats.totalTokensServed !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Total Tokens Served</div>
                    <div className="mt-1 text-sm font-semibold text-slate-900">
                      {(service.usageStats.totalTokensServed / 1_000_000).toFixed(1)}M
                    </div>
                  </div>
                )}
                {service.usageStats.activeUsers !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Active Users</div>
                    <div className="mt-1 text-sm font-semibold text-slate-900">
                      {service.usageStats.activeUsers.toLocaleString()}
                    </div>
                  </div>
                )}
                {service.usageStats.monthlyRequests !== undefined && (
                  <div className="rounded-lg border border-slate-200 bg-white p-3">
                    <div className="text-xs text-slate-500">Monthly Requests</div>
                    <div className="mt-1 text-sm font-semibold text-slate-900">
                      {(service.usageStats.monthlyRequests / 1000).toFixed(1)}K
                    </div>
                  </div>
                )}
              </div>
            </section>
          )}

          {/* Compliance */}
          {service.compliance && service.compliance.length > 0 && (
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">Compliance Certifications</h3>
              <div className="mt-2 flex flex-wrap gap-2">
                {service.compliance.map((cert) => (
                  <span key={cert} className="rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-700">
                    {cert}
                  </span>
                ))}
              </div>
            </section>
          )}
        </div>

        {/* Footer Actions */}
        <div className="sticky bottom-0 border-t border-slate-200 bg-white px-6 py-4">
          <div className="flex items-center justify-end gap-3">
            <button
              onClick={onClose}
              className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Close
            </button>
            <button
              onClick={() => onStartUsing(service)}
              className="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
            >
              Start Using
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
