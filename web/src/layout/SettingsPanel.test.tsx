import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import SettingsPanel from './SettingsPanel'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

test('loads settings and shows save success state', async () => {
  const fetchMock = vi
    .spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(
      jsonResponse({ default_project_dir: 'C:\\Users\\Fireurza\\Documents\\Code' }),
    )
    .mockResolvedValueOnce(
      jsonResponse({ available: false, reason: '', sources: [], agents: [], providers: [], models: [], mcpServers: [], skills: [], plugins: [], shellProfiles: [], warnings: [] }),
    )
    .mockResolvedValueOnce(jsonResponse({ ok: true }))

  render(<SettingsPanel />)

  const input = await screen.findByTestId('settings-default-project-dir')
  expect(input).toHaveValue('C:\\Users\\Fireurza\\Documents\\Code')

  await userEvent.clear(input)
  await userEvent.type(input, 'G:\\My Drive\\GlyphDeck')
  await userEvent.click(screen.getByTestId('settings-save-button'))

  await waitFor(() => {
    expect(screen.getByTestId('settings-message')).toHaveTextContent(
      'Settings saved.',
    )
  })
  expect(fetchMock).toHaveBeenLastCalledWith('/api/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: expect.stringContaining('G:\\\\My Drive\\\\GlyphDeck'),
  })
})
