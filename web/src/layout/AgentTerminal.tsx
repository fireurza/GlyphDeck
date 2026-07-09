import { useEffect, useRef, useState } from 'react'
import type { EventStreamStatus, StreamEvent } from '../api/events'

interface AgentTerminalProps {
  selectedProjectId: string | null
  selectedSessionId: string | null
  eventStreamStatus: EventStreamStatus
  latestEvent: StreamEvent | null
}

type Category = 'tool' | 'shell' | 'message' | 'permission' | 'system'
type FilterValue = 'all' | Category

interface EventRow {
  key: string
  ts: number
  type: string
  category: Category
  label: string
  summary: string
}

const MAX_ROWS = 300

const FILTERS: { value: FilterValue; label: string; testId: string }[] = [
  { value: 'all', label: 'All', testId: 'agent-terminal-filter-all' },
  { value: 'tool', label: 'Tool', testId: 'agent-terminal-filter-tool' },
  { value: 'shell', label: 'Shell', testId: 'agent-terminal-filter-shell' },
  { value: 'message', label: 'Message', testId: 'agent-terminal-filter-message' },
  { value: 'permission', label: 'Permission', testId: 'agent-terminal-filter-permission' },
  { value: 'system', label: 'System', testId: 'agent-terminal-filter-system' },
]

/** Classify a normalized event type into an Agent Terminal filter bucket. */
function classify(type: string): Category {
  if (type.startsWith('glyphdeck.') || type === 'opencode.unknown') return 'system'
  if (type.includes('.tool.')) return 'tool'
  if (type.includes('.shell.')) return 'shell'
  if (type.startsWith('opencode.permission.')) return 'permission'
  return 'message'
}

const LABELS: Record<string, string> = {
  'opencode.session.updated': 'Session updated',
  'opencode.message.updated': 'Message updated',
  'opencode.message.part.updated': 'Text update',
  'opencode.message.part.delta': 'Streaming',
  'opencode.session.next.step.started': 'Step started',
  'opencode.session.next.step.ended': 'Step ended',
  'opencode.session.next.step.failed': 'Step failed',
  'opencode.session.next.tool.called': 'Tool called',
  'opencode.session.next.tool.progress': 'Tool progress',
  'opencode.session.next.tool.success': 'Tool success',
  'opencode.session.next.tool.failed': 'Tool failed',
  'opencode.session.next.shell.started': 'Shell started',
  'opencode.session.next.shell.ended': 'Shell ended',
  'opencode.permission.asked': 'Permission asked',
  'opencode.permission.replied': 'Permission replied',
  'opencode.session.status': 'Session status',
  'glyphdeck.eventstream.connected': 'Stream connected',
  'glyphdeck.eventstream.disconnected': 'Stream disconnected',
  'glyphdeck.eventstream.error': 'Stream error',
}

function shortText(s: unknown, max = 90): string {
  if (typeof s !== 'string') return ''
  const trimmed = s.trim()
  return trimmed.length > max ? trimmed.slice(0, max) + '…' : trimmed
}

/** Build a concise, defect-tolerant payload summary for a normalized event. */
function summarize(type: string, payload: Record<string, unknown>): string {
  const p = payload || {}
  switch (type) {
    case 'opencode.message.part.delta':
      return shortText(p.delta)
    case 'opencode.message.part.updated': {
      const part = p.part as Record<string, unknown> | undefined
      if (part && part.type === 'text') return shortText(part.text)
      return part ? String(part.type ?? '') : ''
    }
    case 'opencode.message.updated': {
      const info = p.info as Record<string, unknown> | undefined
      return info ? `role: ${String(info.role ?? 'unknown')}` : ''
    }
    case 'opencode.session.updated': {
      const info = p.info as Record<string, unknown> | undefined
      return info ? shortText(info.title) : ''
    }
    case 'opencode.session.next.step.started':
      return `agent: ${String(p.agent ?? '')}`
    case 'opencode.session.next.step.ended':
      return `finish: ${String(p.finish ?? '')}`
    case 'opencode.session.next.step.failed':
      return shortText(JSON.stringify(p.error ?? {}))
    case 'opencode.session.next.tool.called':
      return `${String(p.tool ?? '')} ${shortText(JSON.stringify(p.input ?? {}), 60)}`
    case 'opencode.session.next.tool.progress':
      return 'in progress…'
    case 'opencode.session.next.tool.success':
      return `${String(p.tool ?? 'tool')} succeeded`
    case 'opencode.session.next.tool.failed':
      return shortText(JSON.stringify(p.error ?? {}))
    case 'opencode.session.next.shell.started':
      return shortText(p.command)
    case 'opencode.session.next.shell.ended':
      return shortText(p.output)
    case 'opencode.permission.asked':
      return `permission: ${String(p.permission ?? '')}`
    case 'opencode.permission.replied':
      return `reply: ${String(p.reply ?? '')}`
    default:
      try {
        return shortText(JSON.stringify(p))
      } catch {
        return ''
      }
  }
}

/**
 * Events for which repeated occurrences of the SAME message/call should
 * collapse into one updating row instead of growing the log per token/chunk.
 */
function collapseKey(ev: StreamEvent): string | null {
  const p = ev.payload as Record<string, unknown>
  if (ev.type === 'opencode.message.part.delta') {
    return `delta:${String(p.messageID ?? '')}`
  }
  if (ev.type === 'opencode.message.part.updated') {
    const part = p.part as Record<string, unknown> | undefined
    return `part:${String(part?.messageID ?? '')}`
  }
  if (ev.type === 'opencode.session.next.tool.progress') {
    return `progress:${String(p.callID ?? '')}`
  }
  return null
}

function AgentTerminal({
  selectedProjectId,
  selectedSessionId,
  eventStreamStatus,
  latestEvent,
}: AgentTerminalProps) {
  const [rows, setRows] = useState<EventRow[]>([])
  const [filter, setFilter] = useState<FilterValue>('all')
  const rowIndexByKey = useRef<Map<string, number>>(new Map())
  const seq = useRef(0)

  // Fresh log per session — avoid showing a prior session's activity under
  // the newly selected one.
  useEffect(() => {
    setRows([])
    rowIndexByKey.current = new Map()
  }, [selectedProjectId, selectedSessionId])

  useEffect(() => {
    const ev = latestEvent
    if (!ev) return
    if (!selectedProjectId || ev.projectId !== selectedProjectId) return

    // Session-scoped events must belong to the active session. Events with
    // no sessionId (system/eventstream signals) are always shown.
    if (ev.sessionId && ev.sessionId !== selectedSessionId) return

    const category = classify(ev.type)
    const label = LABELS[ev.type] ?? ev.type
    const summary = summarize(ev.type, ev.payload)
    const dedupeKey = collapseKey(ev)

    setRows((prev) => {
      if (dedupeKey) {
        const existingIdx = rowIndexByKey.current.get(dedupeKey)
        if (existingIdx !== undefined && prev[existingIdx]) {
          const next = [...prev]
          next[existingIdx] = { ...next[existingIdx], ts: Date.now(), summary }
          return next
        }
      }

      const row: EventRow = {
        key: `evt-${seq.current++}`,
        ts: Date.now(),
        type: ev.type,
        category,
        label,
        summary,
      }
      let next = [...prev, row]
      if (dedupeKey) {
        rowIndexByKey.current.set(dedupeKey, next.length - 1)
      }
      if (next.length > MAX_ROWS) {
        const dropped = next.length - MAX_ROWS
        next = next.slice(dropped)
        // Re-index after trimming from the front.
        const reindexed = new Map<string, number>()
        rowIndexByKey.current.forEach((idx, k) => {
          const newIdx = idx - dropped
          if (newIdx >= 0) reindexed.set(k, newIdx)
        })
        rowIndexByKey.current = reindexed
      }
      return next
    })
  }, [latestEvent, selectedProjectId, selectedSessionId])

  function handleClear() {
    setRows([])
    rowIndexByKey.current = new Map()
  }

  const visibleRows = filter === 'all' ? rows : rows.filter((r) => r.category === filter)
  const hasActiveSession = !!(selectedProjectId && selectedSessionId)

  return (
    <div className="agent-terminal" data-testid="agent-terminal-panel">
      <div className="agent-terminal__toolbar">
        <div className="agent-terminal__filters">
          {FILTERS.map((f) => (
            <button
              key={f.value}
              type="button"
              className={`agent-terminal__filter-btn ${filter === f.value ? 'active' : ''}`}
              onClick={() => setFilter(f.value)}
              data-testid={f.testId}
            >
              {f.label}
            </button>
          ))}
        </div>
        <button
          type="button"
          className="agent-terminal__clear-btn"
          onClick={handleClear}
          data-testid="agent-terminal-clear-button"
        >
          Clear
        </button>
      </div>

      <div className="agent-terminal__stream-status">
        Event stream: {eventStreamStatus}
      </div>

      <div className="agent-terminal__log">
        {!hasActiveSession ? (
          <p className="project-message" data-testid="agent-terminal-empty-state">
            Select a project and session to see agent activity.
          </p>
        ) : visibleRows.length === 0 ? (
          <p className="project-message" data-testid="agent-terminal-empty-state">
            No agent activity yet.
          </p>
        ) : (
          visibleRows.map((row) => (
            <div
              key={row.key}
              className={`agent-terminal__row agent-terminal__row--${row.category}`}
              data-testid="agent-terminal-event-row"
            >
              <span className="agent-terminal__row-ts">
                {new Date(row.ts).toLocaleTimeString()}
              </span>
              <span className="agent-terminal__row-type" data-testid="agent-terminal-event-type">
                {row.label}
              </span>
              <span
                className="agent-terminal__row-summary"
                data-testid="agent-terminal-event-summary"
              >
                {row.summary}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

export default AgentTerminal
