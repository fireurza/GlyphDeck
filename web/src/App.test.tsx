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

vi.mock('./layout/ServersPanel', () => ({
  default: () => (
    <aside data-testid="left-panel-body" />
  ),
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

let fetchSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(jsonResponse({}))
})

afterEach(() => {
  fetchSpy.mockRestore()
})

test('renders shell layout', () => {
  render(<App />)
  expect(screen.getByTestId('app-shell')).toBeInTheDocument()
  expect(screen.getByTestId('top-bar')).toBeInTheDocument()
  expect(screen.getByTestId('left-panel-body')).toBeInTheDocument()
  expect(screen.getByTestId('center-panel')).toBeInTheDocument()
  expect(screen.getByTestId('right-panel')).toBeInTheDocument()
  expect(screen.getByTestId('bottom-panel')).toBeInTheDocument()
})

test('opens and closes Settings as a modal dialog', async () => {
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
