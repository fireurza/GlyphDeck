import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SettingsPanel from './SettingsPanel'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

function baseInventory() {
  return {
    available: true, reason: '', sources: [], agents: [], providers: [], models: [],
    mcpServers: [], skills: [], plugins: [], shellProfiles: [], warnings: [],
  }
}

afterEach(() => {
  vi.restoreAllMocks()
})

test('loads settings and shows save success state', async () => {
  const fetchMock = vi
    .spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse({ default_project_dir: 'C:\\Users\\Fireurza\\Documents\\Code' }))
    .mockResolvedValueOnce(jsonResponse(baseInventory()))
    .mockResolvedValueOnce(jsonResponse({ ok: true }))

  render(<SettingsPanel />)

  const input = await screen.findByTestId('settings-default-project-dir')
  expect(input).toHaveValue('C:\\Users\\Fireurza\\Documents\\Code')

  await userEvent.clear(input)
  await userEvent.type(input, 'G:\\My Drive\\GlyphDeck')
  await userEvent.click(screen.getByTestId('settings-save-button'))

  await waitFor(() => {
    expect(screen.getByTestId('settings-message')).toHaveTextContent('Settings saved.')
  })
  expect(fetchMock).toHaveBeenLastCalledWith('/api/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: expect.stringContaining('G:\\\\My Drive\\\\GlyphDeck'),
  })
})

test('shows providers and models from config', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse({
      ...baseInventory(),
      providers: [{ id: 'test-provider', scope: 'global', name: 'Test', enabled: true }],
      models: [{ id: 'test-model', provider: 'test-provider', scope: 'global', enabled: true }],
    }))

  render(<SettingsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('settings-config-providers')).toBeInTheDocument()
  }, { timeout: 3000 })
  expect(screen.getByTestId('settings-config-models')).toBeInTheDocument()
})

test('shows configuration sources', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse({
      ...baseInventory(),
      sources: [{ path: '/home/user/.config/opencode/opencode.jsonc', scope: 'global', format: 'jsonc', loaded: true }],
    }))

  render(<SettingsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('settings-config-sources')).toBeInTheDocument()
  }, { timeout: 3000 })
})

test('shows parse warnings', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse({
      ...baseInventory(),
      warnings: [{ source: 'opencode.json', message: 'parse error: unexpected token' }],
    }))

  render(<SettingsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('settings-config-warnings')).toBeInTheDocument()
  }, { timeout: 3000 })
})

test('no sensitive values in config section', async () => {
  vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse({ ...baseInventory(), available: false }))

  render(<SettingsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('settings-config-unavailable')).toBeInTheDocument()
  }, { timeout: 3000 })
  const html = document.body.innerHTML
  expect(html).not.toContain('API_KEY')
  expect(html).not.toContain('sk-')
})
