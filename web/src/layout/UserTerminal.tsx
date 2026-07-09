import { useCallback, useEffect, useRef, useState } from 'react'
import {
  startTerminal,
  streamTerminal,
  sendTerminalInput,
  closeTerminal,
} from '../api/terminal'

interface UserTerminalProps {
  selectedProjectId: string | null
}

type TermState = 'empty' | 'starting' | 'running' | 'closed' | 'error'

function UserTerminal({ selectedProjectId }: UserTerminalProps) {
  const [state, setState] = useState<TermState>('empty')
  const [terminalId, setTerminalId] = useState<string | null>(null)
  const [outputLines, setOutputLines] = useState<string[]>([])
  const [inputText, setInputText] = useState('')
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)
  const outputEndRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const outputBufferRef = useRef<string>('')

  const appendOutput = useCallback((text: string) => {
    outputBufferRef.current += text
    // Flush on newlines or when buffer exceeds threshold.
    if (text.includes('\n') || outputBufferRef.current.length > 2000) {
      flushBuffer()
    }
  }, [])

  const flushBuffer = useCallback(() => {
    if (!outputBufferRef.current) return
    setOutputLines((prev) => {
      const lines = outputBufferRef.current.split('\n')
      outputBufferRef.current = lines.pop() || ''
      return [...prev, ...lines]
    })
  }, [])

  /* Periodic flush for terminal output without newlines. */
  useEffect(() => {
    const interval = setInterval(() => flushBuffer(), 500)
    return () => clearInterval(interval)
  }, [flushBuffer])

  /* Auto-scroll. */
  useEffect(() => {
    outputEndRef.current?.scrollIntoView({ behavior: 'auto' })
  }, [outputLines])

  /* Start terminal. */
  async function handleStart() {
    if (!selectedProjectId) return
    setState('starting')
    setErrorMsg(null)
    setOutputLines([])
    outputBufferRef.current = ''

    try {
      // Use project path as cwd. For the POC, the project path
      // is stored in the UI from the LeftPanel. We approximate it
      // from the selected project context.
      // The simplest approach: use "." as cwd — the backend
      // will resolve to the project's registered path.
      const status = await startTerminal(selectedProjectId, '.')
      setTerminalId(status.id)
      setState('running')

      // Start SSE stream for output.
      const ctrl = new AbortController()
      abortRef.current = ctrl

      streamTerminal(
        status.id,
        (text) => appendOutput(text),
        () => setState('closed'),
        ctrl.signal,
      )
    } catch (err) {
      setState('error')
      setErrorMsg(
        err instanceof Error ? err.message : 'Could not start terminal.',
      )
    }
  }

  /* Send input. */
  async function handleSendInput(e: React.FormEvent) {
    e.preventDefault()
    if (!terminalId || !inputText.trim()) return

    const cmd = inputText + '\r\n'
    setInputText('')
    appendOutput(`> ${inputText}\n`)

    try {
      await sendTerminalInput(terminalId, cmd)
    } catch {
      appendOutput('[input error]\n')
    }
  }

  /* Close terminal. */
  async function handleClose() {
    if (!terminalId) return
    abortRef.current?.abort()
    try {
      await closeTerminal(terminalId)
    } catch {
      /* Best-effort. */
    }
    // Flush any remaining buffered text.
    if (outputBufferRef.current) {
      setOutputLines((prev) => [...prev, outputBufferRef.current])
      outputBufferRef.current = ''
    }
    setState('closed')
    setTerminalId(null)
  }

  /* Cleanup on unmount or project change. */
  useEffect(() => {
    return () => {
      if (terminalId) {
        abortRef.current?.abort()
        closeTerminal(terminalId).catch(() => {})
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedProjectId])

  if (!selectedProjectId) {
    return (
      <div className="panel-body panel-placeholder" data-testid="user-terminal-empty-state">
        <p>No project selected.</p>
        <p className="panel-hint">Select a project to open a terminal.</p>
      </div>
    )
  }

  return (
    <div className="user-terminal" data-testid="user-terminal-panel">
      {/* Toolbar */}
      <div className="user-terminal__toolbar">
        <span className="user-terminal__title">Terminal</span>
        <span className="user-terminal__status" data-testid="user-terminal-status">
          {state === 'running' ? 'Running' : state === 'starting' ? 'Starting…' : state === 'closed' ? 'Closed' : 'Idle'}
        </span>
        <div className="user-terminal__actions">
          {state === 'empty' || state === 'closed' || state === 'error' ? (
            <button
              className="user-terminal__btn user-terminal__btn--start"
              onClick={handleStart}
              data-testid="user-terminal-start-button"
            >
              Start Terminal
            </button>
          ) : null}
          {state === 'running' && (
            <button
              className="user-terminal__btn user-terminal__btn--close"
              onClick={handleClose}
              data-testid="user-terminal-close-button"
            >
              Close
            </button>
          )}
        </div>
      </div>

      {/* Error state */}
      {state === 'error' && (
        <p
          className="project-message project-message--error"
          role="alert"
          data-testid="user-terminal-error"
        >
          {errorMsg}
        </p>
      )}

      {/* Terminal viewport */}
      {(state === 'starting' || state === 'running' || state === 'closed') && (
        <div
          className="user-terminal__viewport"
          data-testid="user-terminal-viewport"
        >
          <div
            className="user-terminal__output"
            data-testid="user-terminal-output"
          >
            {state === 'starting' ? (
              <span className="user-terminal__output-line user-terminal__output-line--hint">
                Starting terminal…
              </span>
            ) : outputLines.length === 0 ? (
              <span className="user-terminal__output-line user-terminal__output-line--hint">
                {state === 'running'
                  ? 'Terminal started. Type a command.'
                  : 'Terminal session ended.'}
              </span>
            ) : (
              outputLines.map((line, i) => (
                <span
                  key={i}
                  className="user-terminal__output-line"
                  data-testid="user-terminal-output-line"
                >
                  {line}
                </span>
              ))
            )}
            <div ref={outputEndRef} />
          </div>
        </div>
      )}

      {/* Input line */}
      {state === 'running' && (
        <form className="user-terminal__input-bar" onSubmit={handleSendInput}>
          <span className="user-terminal__prompt">&gt;</span>
          <input
            ref={inputRef}
            className="user-terminal__input"
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder="Type a command…"
            data-testid="user-terminal-input"
            autoFocus
          />
        </form>
      )}
    </div>
  )
}

export default UserTerminal
