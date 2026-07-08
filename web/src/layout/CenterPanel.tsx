import { useEffect, useRef, useState, type FormEvent } from 'react'
import { fetchMessages, sendPrompt } from '../api/sessions'
import type { GlyphMessage } from '../types/session'

interface CenterPanelProps {
  selectedProjectId?: string | null
  selectedSessionId?: string | null
}

function CenterPanel({
  selectedProjectId,
  selectedSessionId,
}: CenterPanelProps) {
  const [messages, setMessages] = useState<GlyphMessage[]>([])
  const [promptText, setPromptText] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [isLoadingMessages, setIsLoadingMessages] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const transcriptEndRef = useRef<HTMLDivElement | null>(null)

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
            loadError instanceof Error ? loadError.message : 'Could not load messages.',
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

  useEffect(() => {
    transcriptEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

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
      <div className="center-panel__session-header" data-testid="active-session-heading">
        Session: {selectedSessionId}
      </div>

      <div className="center-panel__transcript" data-testid="transcript">
        {isLoadingMessages ? (
          <p className="project-message">Loading messages…</p>
        ) : messages.length === 0 ? (
          <p className="project-message">No messages yet. Send a prompt to start.</p>
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
                data-testid={`transcript-${role}-message`}
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
