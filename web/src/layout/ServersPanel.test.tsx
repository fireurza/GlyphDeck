import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ServersPanel from './ServersPanel'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

const defaultConfigsResponse = () =>
  jsonResponse({
    configs: [
      {
        id: 'ssh-1',
        name: 'My Server',
        type: 'ssh_alias',
        url: '',
        sshAlias: 'myserver',
        workingDir: '~/projects',
        startCommand: '',
        stopCommand: '',
        statusCommand: '',
        lastPid: 0,
        lastUrl: '',
        lastStatus: 'offline',
        lastCheckedAt: '',
        startedByGlyphdeck: false,
      },
    ],
  })

const emptyConfigsResponse = () => jsonResponse({ configs: [] })

const activeServerResponse = () =>
  jsonResponse({ serverId: '', baseUrl: '', attached: false })

// ---- Empty state ----
test('renders empty servers view', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(emptyConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('left-panel-body')
  expect(screen.getByText('No servers configured.')).toBeInTheDocument()
})

// ---- Add flow ----
test('adds an SSH target', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(emptyConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockResolvedValueOnce(jsonResponse({ id: 'srv-new', name: 'New SSH' }, { status: 201 }))
    .mockResolvedValueOnce(jsonResponse({
      configs: [
        { id: 'srv-new', name: 'New SSH', type: 'ssh_alias', sshAlias: 'newbox', lastPid: 0, lastStatus: 'unknown' },
      ],
    }))
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('left-panel-body')

  await userEvent.click(screen.getByTestId('server-add-button'))
  await screen.findByTestId('server-add-form')

  await userEvent.type(screen.getByTestId('server-add-name'), 'New SSH')
  await userEvent.selectOptions(screen.getByTestId('server-add-type'), 'ssh_alias')
  await userEvent.type(screen.getByTestId('server-add-ssh-alias'), 'newbox')

  await userEvent.click(screen.getByTestId('server-add-submit'))

  await screen.findByTestId('server-card-srv-new')
  expect(screen.getByText('New SSH')).toBeInTheDocument()
})

// ---- Validation ----
test('shows validation error for missing name', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(emptyConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('left-panel-body')

  await userEvent.click(screen.getByTestId('server-add-button'))
  await screen.findByTestId('server-add-form')

  await userEvent.click(screen.getByTestId('server-add-submit'))

  // Error appears after state update (async).
  await waitFor(() => {
    expect(screen.getByText('Name is required')).toBeInTheDocument()
  })
})

test('shows validation error for missing SSH alias', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(emptyConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('left-panel-body')

  await userEvent.click(screen.getByTestId('server-add-button'))
  await screen.findByTestId('server-add-form')

  await userEvent.type(screen.getByTestId('server-add-name'), 'Test')
  await userEvent.selectOptions(screen.getByTestId('server-add-type'), 'ssh_alias')
  await userEvent.click(screen.getByTestId('server-add-submit'))

  await waitFor(() => {
    expect(screen.getByText('SSH alias is required')).toBeInTheDocument()
  })
})

// ---- Edit flow ----
test('edits and saves a target', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockResolvedValueOnce(jsonResponse({ ok: true }))
    .mockResolvedValueOnce(jsonResponse({
      configs: [
        { id: 'ssh-1', name: 'Renamed', type: 'ssh_alias', sshAlias: 'myserver', lastPid: 0, lastStatus: 'unknown' },
      ],
    }))
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('edit-ssh-1'))
  await screen.findByTestId('edit-form-ssh-1')

  const nameInput = screen.getByTestId('edit-name-ssh-1')
  await userEvent.clear(nameInput)
  await userEvent.type(nameInput, 'Renamed')
  await userEvent.click(screen.getByTestId('edit-save-ssh-1'))

  await screen.findByText('Renamed')
})

test('cancel edit discards changes', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('edit-ssh-1'))
  await screen.findByTestId('edit-form-ssh-1')

  const nameInput = screen.getByTestId('edit-name-ssh-1')
  await userEvent.clear(nameInput)
  await userEvent.type(nameInput, 'Changed')
  await userEvent.click(screen.getByTestId('edit-cancel-ssh-1'))

  expect(screen.getByText('My Server')).toBeInTheDocument()
})

// ---- Delete flow ----
test('delete confirmation and cancellation', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('remove-server-ssh-1'))
  await screen.findByTestId('delete-confirm-ssh-1')

  await userEvent.click(screen.getByTestId('delete-confirm-no-ssh-1'))

  expect(screen.getByTestId('server-card-ssh-1')).toBeInTheDocument()
})

test('delete confirms and removes target', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse({
      configs: [
        { id: 'ssh-1', name: 'My Server', type: 'ssh_alias', sshAlias: 'myserver', lastPid: 0, lastStatus: 'unknown' },
        { id: 'ssh-2', name: 'Other', type: 'local', url: '', sshAlias: '', lastPid: 0, lastStatus: 'unknown' },
      ],
    }))
    .mockResolvedValueOnce(activeServerResponse())
    .mockResolvedValueOnce(new Response(null, { status: 204 }))
    .mockResolvedValueOnce(jsonResponse({
      configs: [{ id: 'ssh-2', name: 'Other', type: 'local', lastPid: 0, lastStatus: 'unknown' }],
    }))
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('remove-server-ssh-1'))
  await screen.findByTestId('delete-confirm-ssh-1')
  await userEvent.click(screen.getByTestId('delete-confirm-yes-ssh-1'))

  await waitFor(() => {
    expect(screen.queryByTestId('server-card-ssh-1')).not.toBeInTheDocument()
  })
  expect(screen.getByTestId('server-card-ssh-2')).toBeInTheDocument()
})

// ---- Lifecycle: Test SSH ----
test('Test SSH button triggers action and shows message', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockResolvedValueOnce(jsonResponse({ success: true, message: 'SSH connection OK' }))

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('test-ssh-ssh-1'))

  await waitFor(() => {
    expect(screen.getByTestId('action-msg-ssh-1')).toHaveTextContent('SSH connection OK')
  })
})

// ---- Lifecycle: Detect ----
test('Detect button shows result message', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockResolvedValueOnce(jsonResponse({ status: 'online', message: 'detect completed' }))

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('detect-ssh-1'))

  await waitFor(() => {
    expect(screen.getByTestId('action-msg-ssh-1')).toHaveTextContent('detect completed')
  })
})

// ---- Lifecycle: Start ----
test('Start button shows start result', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockResolvedValueOnce(jsonResponse({ success: true, pid: 111, url: 'http://localhost:4096', message: 'started' }))
    .mockResolvedValueOnce(jsonResponse({
      configs: [{ id: 'ssh-1', name: 'My Server', type: 'ssh_alias', sshAlias: 'myserver', lastPid: 111, lastUrl: 'http://localhost:4096', lastStatus: 'online', startedByGlyphdeck: true }],
    }))
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('start-remote-ssh-1'))

  await waitFor(() => {
    expect(screen.getByTestId('action-msg-ssh-1')).toHaveTextContent('started')
  })
})

// ---- Lifecycle: Stop ownership ----
test('Stop button disabled without owned PID', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  expect(screen.getByTestId('stop-remote-ssh-1')).toBeDisabled()
})

test('Stop button enabled with owned PID', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse({
      configs: [
        { id: 'ssh-1', name: 'My Server', type: 'ssh_alias', sshAlias: 'myserver', lastPid: 12345, lastStatus: 'online', startedByGlyphdeck: true },
      ],
    }))
    .mockResolvedValueOnce(activeServerResponse())

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  expect(screen.getByTestId('stop-remote-ssh-1')).not.toBeDisabled()
})

// ---- Attach / Detach ----
test('Detach button clears active server', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(jsonResponse({ serverId: 'ssh-1', baseUrl: 'http://example.com:4096', attached: true }))
    .mockResolvedValueOnce(jsonResponse({ attached: false }))

  render(<ServersPanel />)
  await screen.findByTestId('active-server-banner')

  const detachBtn = screen.getByTestId('detach-server-button')
  expect(detachBtn).toBeInTheDocument()

  await userEvent.click(detachBtn)

  await waitFor(() => {
    expect(screen.getByTestId('no-active-server')).toBeInTheDocument()
  })
})

// ---- Error display ----
test('shows and dismisses per-target error', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockRejectedValueOnce(new Error('Connection refused'))

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('test-ssh-ssh-1'))

  await waitFor(() => {
    expect(screen.getByTestId('error-msg-ssh-1')).toBeInTheDocument()
  })

  await userEvent.click(screen.getByTestId('dismiss-error-ssh-1'))
  expect(screen.queryByTestId('error-msg-ssh-1')).not.toBeInTheDocument()
})

// ---- Operation states ----
test('shows transient operation state during SSH test', async () => {
  let resolveOp!: (value: Response) => void
  const opPromise = new Promise<Response>((resolve) => {
    resolveOp = resolve
  })

  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockImplementationOnce(() => opPromise)

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('test-ssh-ssh-1'))

  await screen.findByTestId('op-state-ssh-1')
  expect(screen.getByTestId('op-state-ssh-1')).toHaveTextContent('Testing SSH…')

  resolveOp(jsonResponse({ success: true, message: 'OK' }))

  await waitFor(() => {
    expect(screen.getByTestId('action-msg-ssh-1')).toHaveTextContent('OK')
  })
  expect(screen.queryByTestId('op-state-ssh-1')).not.toBeInTheDocument()
})

// ---- Action disable during ops ----
test('edit button disabled during operation', async () => {
  let resolveOp: (value: Response) => void
  const opPromise = new Promise<Response>((resolve) => {
    resolveOp = resolve
  })

  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(defaultConfigsResponse())
    .mockResolvedValueOnce(activeServerResponse())
    .mockImplementationOnce(() => opPromise)

  render(<ServersPanel />)
  await screen.findByTestId('server-card-ssh-1')

  await userEvent.click(screen.getByTestId('test-ssh-ssh-1'))

  await screen.findByTestId('op-state-ssh-1')

  expect(screen.getByTestId('edit-ssh-1')).toBeDisabled()
  expect(screen.getByTestId('remove-server-ssh-1')).toBeDisabled()

  resolveOp!(jsonResponse({ success: true, message: 'OK' }))
  await waitFor(() => {
    expect(screen.getByTestId('action-msg-ssh-1')).toHaveTextContent('OK')
  })
})
