import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { fetchMessages, sendPrompt } from '../api/sessions'
import type {
  DeltaPayload,
  EventStreamStatus,
  PartUpdatedPayload,
  StreamEvent,
} from '../api/events'
import type { GlyphMessage, GlyphPart } from '../types/session'

interface CenterPanelProps {
  selectedProjectId?: string | null
  selectedSessionId?: string | null
  eventStreamStatus: EventStreamStatus
  latestEvent: StreamEvent | null
}

function CenterPanel({
  selectedProjectId,
  selectedSessionId,
  eventStreamStatus,
  latestEvent,
}: CenterPanelProps) {
  const [messages, setMessages] = useState<GlyphMessage[]>([])
  const [promptText, setPromptText] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [isLoadingMessages, setIsLoadingMessages] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const transcriptEndRef = useRef<HTMLDivElement | null>(null)

  /* ---- event-stream state ---- */
  const streamedMessageIds = useRef<Set<string>>(new Set())
  const deltaBufferRef = useRef<
    Record<string, { text: string; role: string }>
  >({})
  const flushTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const eventRefreshControllerRef = useRef<AbortController | null>(null)
  const latestEventRef = useRef(latestEvent)
  latestEventRef.current = latestEvent

  /* clear delta buffer when session changes */
  useEffect(() => {
    deltaBufferRef.current = {}
    streamedMessageIds.current = new Set()
    if (flushTimerRef.current) {
      clearTimeout(flushTimerRef.current)
      flushTimerRef.current = null
    }
    eventRefreshControllerRef.current?.abort()
  }, [selectedSessionId])

  /* ---- load messages on session select ---- */
  useEffect(() => {
    if (!selectedProjectId || !selectedSessionId) {
      setMessages([])
      setError(null)
      return
    }

    const controller = new AbortController()

    async function loadMessages() {
      try {
        setIsLoadingMessages(true)
        setError(null)
        const fetched = await fetchMessages(
          selectedProjectId!,
          selectedSessionId!,
          controller.signal,
        )
        if (!controller.signal.aborted) {
          setMessages(fetched)
        }
      } catch (loadError) {
        if (loadError instanceof DOMException && loadError.name === 'AbortError') {
          return
        }
        if (!controller.signal.aborted) {
          setError(
            loadError instanceof Error
              ? loadError.message
              : 'Could not load messages.',
          )
        }
      } finally {
        if (!controller.signal.aborted) {
          setIsLoadingMessages(false)
        }
      }
    }

    loadMessages()

    return () => controller.abort()
  }, [selectedProjectId, selectedSessionId])

  /* ---- auto-scroll on message changes ---- */
  useEffect(() => {
    transcriptEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  /* ---- event-triggered message refresh ---- */
  const triggerRefresh = useCallback(() => {
    const pid = selectedProjectId
    const sid = selectedSessionId
    if (!pid || !sid) return

    eventRefreshControllerRef.current?.abort()
    const ctrl = new AbortController()
    eventRefreshControllerRef.current = ctrl

    fetchMessages(pid, sid, ctrl.signal)
      .then((fetched) => {
        if (!ctrl.signal.aborted) {
          setMessages(fetched)
        }
      })
      .catch(() => {
        /* Silently ignore refresh errors */
      })
  }, [selectedProjectId, selectedSessionId])

  /* ---- flush buffered deltas into messages state ---- */
  const flushDeltas = useCallback(() => {
    const buffer = { ...deltaBufferRef.current }
    deltaBufferRef.current = {}

    if (Object.keys(buffer).length === 0) return

    setMessages((prev) => {
      const next = [...prev]
      for (const [msgId, buf] of Object.entries(buffer)) {
        streamedMessageIds.current.add(msgId)
        const idx = next.findIndex((m) => m.info.id === msgId)
        if (idx >= 0) {
          const parts: GlyphPart[] = [...next[idx].parts]
          const textIdx = parts.findIndex((p) => p.type === 'text')
          if (textIdx >= 0) {
            parts[textIdx] = { ...parts[textIdx], text: buf.text }
          } else {
            parts.push({ type: 'text', text: buf.text })
          }
          next[idx] = {
            ...next[idx],
            parts,
            info: { ...next[idx].info, role: buf.role || next[idx].info.role },
          }
        } else {
          next.push({
            info: { id: msgId, role: buf.role || 'assistant' },
            parts: [{ type: 'text', text: buf.text }],
          })
        }
      }
      return next
    })
  }, [])

  /* ---- process latest event ---- */
  useEffect(() => {
    const event = latestEventRef.current
    if (!event) return

    const activePid = selectedProjectId
    const activeSid = selectedSessionId
    if (!activePid || !activeSid) return
    if (event.projectId !== activePid || event.sessionId !== activeSid) return

    const scheduleFlush = () => {
      if (!flushTimerRef.current) {
        flushTimerRef.current = setTimeout(() => {
          flushTimerRef.current = null
          flushDeltas()
        }, 100)
      }
    }

    switch (event.type) {
      // Authoritative full-text snapshot of a streamed part. Update the
      // transcript directly from the stream — no fallback fetch required.
      case 'opencode.message.part.updated': {
        const p = event.payload as unknown as PartUpdatedPayload
        const part = p?.part
        if (!part || part.type !== 'text' || typeof part.text !== 'string') break
        if (!part.messageID) break
        deltaBufferRef.current[part.messageID] = {
          text: part.text,
          role: 'assistant',
        }
        scheduleFlush()
        break
      }

      // Incremental text delta — append to the buffered streamed text.
      case 'opencode.message.part.delta': {
        const p = event.payload as unknown as DeltaPayload
        if (p?.field !== 'text' || !p.messageID || typeof p.delta !== 'string') {
          break
        }
        const entry = deltaBufferRef.current[p.messageID]
        if (entry) {
          entry.text += p.delta
        } else {
          deltaBufferRef.current[p.messageID] = {
            text: p.delta,
            role: 'assistant',
          }
        }
        scheduleFlush()
        break
      }

      // Message-level metadata change — reconcile roles/parts from the API.
      // This is reconciliation only; streaming proof comes from the part
      // events above.
      case 'opencode.message.updated':
        triggerRefresh()
        break
    }
  }, [latestEvent, selectedProjectId, selectedSessionId, triggerRefresh, flushDeltas])

  /* ---- send prompt ---- */
  async function handleSendPrompt(event: FormEvent) {
    event.preventDefault()

    const trimmed = promptText.trim()
    if (!trimmed || !selectedProjectId || !selectedSessionId || isSending) {
      return
    }

    try {
      setIsSending(true)
      setError(null)
      await sendPrompt(selectedProjectId, selectedSessionId, { text: trimmed })
      setPromptText('')

      const updated = await fetchMessages(selectedProjectId, selectedSessionId)
      setMessages(updated)
    } catch (sendError) {
      setError(
        sendError instanceof Error ? sendError.message : 'Could not send prompt.',
      )
    } finally {
      setIsSending(false)
    }
  }

  function getRoleLabel(role: string) {
    return role === 'user' ? 'You' : 'Assistant'
  }

  function extractText(parts: GlyphMessage['parts']) {
    return parts
      .filter((part) => part.type === 'text' && typeof part.text === 'string')
      .map((part) => part.text!)
      .join('\n')
  }

  function getMessageTestId(msg: GlyphMessage) {
    if (streamedMessageIds.current.has(msg.info.id)) {
      return 'transcript-streamed-message'
    }
    const role = msg.info.role === 'user' ? 'user' : 'assistant'
    return `transcript-${role}-message`
  }

  const hasActiveSession = selectedProjectId && selectedSessionId

  if (!hasActiveSession) {
    return (
      <main className="center-panel">
        <div className="panel-body panel-placeholder">
          <h2>Welcome to GlyphDeck</h2>
          <p>Your local OpenCode workspace dashboard.</p>
          <p className="panel-hint">
            Select a project and session to get started.
          </p>
        </div>
      </main>
    )
  }

  return (
    <main className="center-panel">
      <div
        className="center-panel__session-header"
        data-testid="active-session-heading"
      >
        Session: {selectedSessionId}
        {eventStreamStatus === 'connected' && (
          <span
            className="transcript-live-indicator"
            data-testid="transcript-live-indicator"
          >
            Live
          </span>
        )}
      </div>

      <div className="center-panel__transcript" data-testid="transcript">
        {isLoadingMessages ? (
          <p className="project-message">Loading messages…</p>
        ) : messages.length === 0 ? (
          <p className="project-message">
            No messages yet. Send a prompt to start.
          </p>
        ) : (
          messages.map((msg) => {
            const role = msg.info.role

            return (
              <div
                key={msg.info.id}
                className={
                  'transcript-message transcript-message--' +
                  (role === 'user' ? 'user' : 'assistant')
                }
                data-testid={getMessageTestId(msg)}
              >
                <div className="transcript-message__role">
                  {getRoleLabel(role)}
                </div>
                <div className="transcript-message__text">
                  {extractText(msg.parts)}
                </div>
              </div>
            )
          })
        )}

        {error ? (
          <p className="project-message project-message--error" role="alert">
            {error}
          </p>
        ) : null}

        {isSending && <p className="project-message">Sending…</p>}

        <div ref={transcriptEndRef} />
      </div>

      <form className="prompt-composer" onSubmit={handleSendPrompt}>
        <textarea
          className="prompt-composer__input"
          value={promptText}
          onChange={(event) => setPromptText(event.target.value)}
          placeholder="Type a prompt..."
          disabled={isSending}
          rows={2}
          data-testid="prompt-composer-input"
        />
        <button
          className="prompt-composer__send"
          type="submit"
          disabled={isSending || promptText.trim().length === 0}
          data-testid="prompt-send-button"
        >
          Send
        </button>
      </form>
    </main>
  )
}

export default CenterPanel
