import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ConfigListPanel from './ConfigListPanel'

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
  })
}

const baseInventory = {
  available: true, reason: '', sources: [], agents: [], providers: [], models: [],
  skills: [], plugins: [], shellProfiles: [], warnings: [],
}

afterEach(() => { vi.restoreAllMocks() })

describe('ConfigListPanel — MCP Servers', () => {
  test('renders populated MCP servers', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse({
      ...baseInventory,
      mcpServers: [
        { name: 'context7', type: 'remote', scope: 'global', url: 'https://mcp.context7.com', enabled: true },
        { name: 'local-server', type: 'local', scope: 'global', command: 'server --port 8080', enabled: true },
      ],
    }))

    render(<ConfigListPanel category="mcpServers" title="MCP Servers" />)
    await waitFor(() => { expect(screen.getByTestId('mcp-list')).toBeInTheDocument() })
    expect(screen.getByTestId('mcp-context7')).toBeInTheDocument()
    expect(screen.getByTestId('mcp-local-server')).toBeInTheDocument()
  })

  test('renders empty state', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse(baseInventory))
    render(<ConfigListPanel category="mcpServers" title="MCP Servers" />)
    await waitFor(() => { expect(screen.getByTestId('mcp-empty')).toBeInTheDocument() })
  })

  test('renders error state', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Failed'))
    render(<ConfigListPanel category="mcpServers" title="MCP Servers" />)
    await waitFor(() => { expect(screen.getByText('Failed')).toBeInTheDocument() })
  })

  test('no sensitive values rendered', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse({
      ...baseInventory,
      mcpServers: [{ name: 'test', type: 'remote', scope: 'global', url: 'https://example.com', enabled: true }],
    }))
    render(<ConfigListPanel category="mcpServers" title="MCP Servers" />)
    await waitFor(() => { expect(screen.getByTestId('mcp-test')).toBeInTheDocument() })
    const html = document.body.innerHTML
    expect(html).not.toContain('API_KEY')
    expect(html).not.toContain('sk-')
    expect(html).not.toContain('token')
  })
})

describe('ConfigListPanel — Skills', () => {
  test('renders populated skills', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse({
      ...baseInventory,
      skills: [
        { name: 'code-review', scope: 'global', description: 'Reviews code', enabled: true },
        { name: 'frontend-design', scope: 'global', enabled: true },
      ],
    }))
    render(<ConfigListPanel category="skills" title="Skills" />)
    await waitFor(() => { expect(screen.getByTestId('skills-list')).toBeInTheDocument() })
    expect(screen.getByTestId('skills-code-review')).toBeInTheDocument()
    expect(screen.getByTestId('skills-frontend-design')).toBeInTheDocument()
  })

  test('renders empty state', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse(baseInventory))
    render(<ConfigListPanel category="skills" title="Skills" />)
    await waitFor(() => { expect(screen.getByTestId('skills-empty')).toBeInTheDocument() })
  })
})

describe('ConfigListPanel — Plugins', () => {
  test('renders populated plugins', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse({
      ...baseInventory,
      plugins: [
        { id: 'caveman', scope: 'global', type: 'local', enabled: true },
        { id: '@cortexkit/opencode-magic-context', scope: 'global', type: 'npm', enabled: true },
      ],
    }))
    render(<ConfigListPanel category="plugins" title="Plugins" />)
    await waitFor(() => { expect(screen.getByTestId('plugins-list')).toBeInTheDocument() })
    expect(screen.getByTestId('plugins-caveman')).toBeInTheDocument()
    expect(screen.getByTestId('plugins-@cortexkit/opencode-magic-context')).toBeInTheDocument()
  })
})

describe('ConfigListPanel — scope filter', () => {
  test('filters by scope', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(jsonResponse({
      ...baseInventory,
      skills: [
        { name: 'global-skill', scope: 'global', enabled: true },
        { name: 'project-skill', scope: 'project', enabled: true },
      ],
    }))

    render(<ConfigListPanel category="skills" title="Skills" selectedProjectId="p1" />)
    await waitFor(() => { expect(screen.getByTestId('skills-list')).toBeInTheDocument() })

    await userEvent.click(screen.getByTestId('skills-filter-global'))
    expect(screen.getByTestId('skills-global-skill')).toBeInTheDocument()
    expect(screen.queryByTestId('skills-project-skill')).not.toBeInTheDocument()
  })
})
