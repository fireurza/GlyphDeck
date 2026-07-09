import { useCallback, useEffect, useRef, useState } from 'react'
import { fetchUsage } from '../api/usage'
import type { UsageResponse } from '../types/usage'

interface UsagePanelProps {
  selectedProjectId?: string | null
  selectedSessionId?: string | null
}

type PanelState = 'empty' | 'loading' | 'error' | 'unavailable' | 'data'

function formatCost(cost: number): string {
  if (cost === 0) return '$0.00'
  // Show up to 6 decimal places for small costs, trimming trailing zeros.
  const fixed = cost.toFixed(6)
  const trimmed = fixed.replace(/0+$/, '').replace(/\.$/, '')
  return `$${trimmed}`
}

function formatTokens(n: number): string {
  if (n === 0) return '0'
  return n.toLocaleString()
}

function UsagePanel({ selectedProjectId, selectedSessionId }: UsagePanelProps) {
  const [data, setData] = useState<UsageResponse | null>(null)
  const [panelState, setPanelState] = useState<PanelState>('empty')
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const loadUsage = useCallback(async () => {
    if (!selectedProjectId || !selectedSessionId) {
      setPanelState('empty')
      setData(null)
      setErrorMessage(null)
      return
    }

    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl

    setPanelState('loading')
    setErrorMessage(null)

    try {
      const result = await fetchUsage(selectedProjectId, selectedSessionId, ctrl.signal)
      if (!ctrl.signal.aborted) {
        setData(result)
        if (result.available) {
          setPanelState('data')
        } else {
          setPanelState('unavailable')
        }
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') return
      if (!ctrl.signal.aborted) {
        setPanelState('error')
        setErrorMessage(err instanceof Error ? err.message : 'Could not load usage data.')
      }
    }
  }, [selectedProjectId, selectedSessionId])

  // Load usage when session changes.
  useEffect(() => {
    loadUsage()
    return () => abortRef.current?.abort()
  }, [loadUsage])

  if (!selectedProjectId || !selectedSessionId) {
    return (
      <div className="panel-body panel-placeholder" data-testid="usage-empty-state">
        <p>No session selected.</p>
        <p className="panel-hint">Select a session to view usage data.</p>
      </div>
    )
  }

  return (
    <div className="usage-panel" data-testid="usage-panel">
      <div className="usage-panel__toolbar">
        <span className="usage-panel__title">Usage</span>
        <button
          className="usage-panel__refresh-btn"
          onClick={loadUsage}
          disabled={panelState === 'loading'}
          data-testid="usage-refresh-button"
        >
          {panelState === 'loading' ? 'Loading…' : 'Refresh'}
        </button>
      </div>

      <div className="panel-body">
        {panelState === 'loading' && (
          <p className="project-message" data-testid="usage-loading-state">
            Loading usage data…
          </p>
        )}

        {panelState === 'error' && (
          <p
            className="project-message project-message--error"
            role="alert"
            data-testid="usage-error-state"
          >
            {errorMessage}
          </p>
        )}

        {panelState === 'unavailable' && (
          <div className="usage-unavailable" data-testid="usage-unavailable-state">
            <p className="usage-unavailable__heading">Usage data unavailable</p>
            <p className="usage-unavailable__reason">
              {data?.reason || 'OpenCode did not provide usage fields for this session yet.'}
            </p>
            {data?.providerID && (
              <p className="usage-unavailable__meta">
                Provider: {data.providerID} / Model: {data.modelID}
              </p>
            )}
          </div>
        )}

        {panelState === 'data' && data && (
          <dl className="usage-stats">
            <div className="usage-stat">
              <dt className="usage-stat__label">Provider</dt>
              <dd className="usage-stat__value" data-testid="usage-provider">
                {data.providerID || '\u2014'}
              </dd>
            </div>
            <div className="usage-stat">
              <dt className="usage-stat__label">Model</dt>
              <dd className="usage-stat__value" data-testid="usage-model">
                {data.modelID || '\u2014'}
              </dd>
            </div>
            {data.agent && (
              <div className="usage-stat">
                <dt className="usage-stat__label">Agent</dt>
                <dd className="usage-stat__value" data-testid="usage-agent">
                  {data.agent}
                </dd>
              </div>
            )}
            {data.mode && (
              <div className="usage-stat">
                <dt className="usage-stat__label">Mode</dt>
                <dd className="usage-stat__value" data-testid="usage-mode">
                  {data.mode}
                </dd>
              </div>
            )}
            <div className="usage-stat">
              <dt className="usage-stat__label">Input Tokens</dt>
              <dd className="usage-stat__value" data-testid="usage-input-tokens">
                {formatTokens(data.tokens.input)}
              </dd>
            </div>
            <div className="usage-stat">
              <dt className="usage-stat__label">Output Tokens</dt>
              <dd className="usage-stat__value" data-testid="usage-output-tokens">
                {formatTokens(data.tokens.output)}
              </dd>
            </div>
            {data.tokens.reasoning > 0 && (
              <div className="usage-stat">
                <dt className="usage-stat__label">Reasoning Tokens</dt>
                <dd className="usage-stat__value" data-testid="usage-reasoning-tokens">
                  {formatTokens(data.tokens.reasoning)}
                </dd>
              </div>
            )}
            <div className="usage-stat">
              <dt className="usage-stat__label">Cache Read</dt>
              <dd className="usage-stat__value" data-testid="usage-cache-read-tokens">
                {formatTokens(data.tokens.cache.read)}
              </dd>
            </div>
            <div className="usage-stat">
              <dt className="usage-stat__label">Cache Write</dt>
              <dd className="usage-stat__value" data-testid="usage-cache-write-tokens">
                {formatTokens(data.tokens.cache.write)}
              </dd>
            </div>
            <div className="usage-stat usage-stat--total">
              <dt className="usage-stat__label">Total Tokens</dt>
              <dd className="usage-stat__value" data-testid="usage-total-tokens">
                {formatTokens(data.tokens.total)}
              </dd>
            </div>
            <div className="usage-stat">
              <dt className="usage-stat__label">Cost</dt>
              <dd className="usage-stat__value" data-testid="usage-cost">
                {formatCost(data.cost)}
              </dd>
            </div>
            <div className="usage-stat">
              <dt className="usage-stat__label">Messages</dt>
              <dd className="usage-stat__value">
                {data.messageCount}
              </dd>
            </div>
          </dl>
        )}
      </div>
    </div>
  )
}

export default UsagePanel
