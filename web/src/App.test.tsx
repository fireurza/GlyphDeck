import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from './App'

vi.mock('./api/events', () => ({
  useEventStream: () => ({ status: 'offline', latestEvent: null }),
}))

vi.mock('./layout/TopBar', () => ({
  default: () => <header data-testid="top-bar" />,
}))

vi.mock('./layout/LeftPanel', () => ({
  default: () => <aside data-testid="left-panel-body" />,
}))

vi.mock('./layout/CenterPanel', () => ({
  default: () => <main data-testid="center-panel" />,
}))

vi.mock('./layout/RightPanel', () => ({
  default: () => <aside data-testid="right-panel" />,
}))

vi.mock('./layout/BottomPanel', () => ({
  default: () => <section data-testid="bottom-panel" />,
}))

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
  })
}

test('opens and closes Settings as a modal dialog', async () => {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({}))

  render(<App />)

  const trigger = screen.getByTestId('activity-settings-button')
  expect(screen.queryByTestId('settings-dialog')).not.toBeInTheDocument()

  await userEvent.click(trigger)
  const dialog = await screen.findByTestId('settings-dialog')
  expect(dialog).toHaveAttribute('open')
  expect(trigger).toHaveAttribute('aria-expanded', 'true')

  await userEvent.click(screen.getByTestId('settings-close-button'))
  await waitFor(() => {
    expect(screen.queryByTestId('settings-dialog')).not.toBeInTheDocument()
  })
  expect(trigger).toHaveAttribute('aria-expanded', 'false')
})
