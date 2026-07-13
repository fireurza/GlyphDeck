import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AgentsPanel from './AgentsPanel'
import type { ConfigInventory } from '../types/config'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

function mockInventory(overrides: Partial<ConfigInventory> = {}): ConfigInventory {
  return {
    available: true,
    reason: '',
    sources: [],
    agents: [],
    providers: [],
    models: [],
    mcpServers: [],
    skills: [],
    plugins: [],
    shellProfiles: [],
    warnings: [],
    ...overrides,
  }
}

afterEach(() => {
  vi.restoreAllMocks()
})

test('renders loading state', () => {
  vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(() => {}))
  render(<AgentsPanel />)
  expect(screen.getByText('Loading agents…')).toBeInTheDocument()
})

test('renders populated agents', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    jsonResponse(
      mockInventory({
        agents: [
          { name: 'builder', scope: 'global', source: 'agents/', enabled: true, role: 'primary' },
          { name: 'reviewer', scope: 'global', source: 'agents/', enabled: true, role: 'subagent', description: 'Code reviewer' },
        ],
      }),
    ),
  )

  render(<AgentsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('agents-list')).toBeInTheDocument()
  })
  expect(screen.getByTestId('agent-builder')).toBeInTheDocument()
  expect(screen.getByTestId('agent-reviewer')).toBeInTheDocument()
  expect(screen.getByTestId('agent-builder-scope')).toHaveTextContent('global')
  expect(screen.getByTestId('agent-reviewer-scope')).toHaveTextContent('global')
})

test('renders empty state', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    jsonResponse(mockInventory()),
  )

  render(<AgentsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('agents-empty')).toBeInTheDocument()
  })
})

test('renders error state', async () => {
  vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'))

  render(<AgentsPanel />)
  await waitFor(() => {
    expect(screen.getByText('Network error')).toBeInTheDocument()
  })
})

test('filters by scope', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    jsonResponse(
      mockInventory({
        agents: [
          { name: 'global-agent', scope: 'global', source: 'agents/', enabled: true },
          { name: 'project-agent', scope: 'project', source: 'agents/', enabled: true },
        ],
      }),
    ),
  )

  render(<AgentsPanel selectedProjectId="proj-1" />)
  await waitFor(() => {
    expect(screen.getByTestId('agents-list')).toBeInTheDocument()
  })

  const globalBtn = screen.getByTestId('agents-filter-global')
  await userEvent.click(globalBtn)

  expect(screen.getByTestId('agent-global-agent')).toBeInTheDocument()
  expect(screen.queryByTestId('agent-project-agent')).not.toBeInTheDocument()
})

test('no sensitive values rendered in agent data', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
    jsonResponse(
      mockInventory({
        agents: [
          { name: 'test-agent', scope: 'global', source: 'agents/', enabled: true, model: 'gpt-4', description: 'Safe description' },
        ],
      }),
    ),
  )

  render(<AgentsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('agent-test-agent')).toBeInTheDocument()
  })

  const html = document.body.innerHTML
  expect(html).not.toContain('api_key')
  expect(html).not.toContain('API_KEY')
  expect(html).not.toContain('sk-')
  expect(html).not.toContain('token')
})

test('refresh reloads agents', async () => {
  const mock = vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(jsonResponse(mockInventory({ agents: [{ name: 'first', scope: 'global', source: 'agents/', enabled: true }] })))
    .mockResolvedValueOnce(jsonResponse(mockInventory({ agents: [{ name: 'second', scope: 'global', source: 'agents/', enabled: true }] })))

  render(<AgentsPanel />)
  await waitFor(() => {
    expect(screen.getByTestId('agent-first')).toBeInTheDocument()
  })

  const refreshBtn = screen.getByTestId('agents-refresh')
  await userEvent.click(refreshBtn)

  await waitFor(() => {
    expect(screen.getByTestId('agent-second')).toBeInTheDocument()
  })
})
