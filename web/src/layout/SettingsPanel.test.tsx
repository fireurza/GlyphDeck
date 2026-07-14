import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import SettingsPanel from "./SettingsPanel"

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), { headers: { "Content-Type": "application/json" }, ...init })
}

function baseDoc(overrides = {}) {
  return { data: { appearance: "system", interfaceDensity: "comfortable", terminalFontSize: 14, transcriptAutoScroll: true, defaultRightPanelTab: "review", destructiveConfirmations: true }, revision: 0, ...overrides }
}

function emptyConfig() {
  return { available: true, reason: "", sources: [], agents: [], providers: [], models: [], mcpServers: [], skills: [], plugins: [], shellProfiles: [], warnings: [] }
}

afterEach(() => {
  vi.restoreAllMocks()
})

test("loads preferences and shows form", async () => {
  vi.spyOn(globalThis, "fetch")
    .mockResolvedValueOnce(jsonResponse({})) // legacy settings
    .mockResolvedValueOnce(jsonResponse(baseDoc())) // preferences GET
    .mockResolvedValueOnce(jsonResponse(emptyConfig())) // config
    .mockResolvedValueOnce(jsonResponse([])) // backups

  render(<SettingsPanel />)
  await waitFor(() => { expect(screen.getByTestId("prefs-appearance")).toBeInTheDocument() }, { timeout: 3000 })
  expect(screen.getByTestId("prefs-appearance")).toHaveValue("system")
})

test("preview and apply changes", async () => {
  vi.spyOn(globalThis, "fetch")
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse(baseDoc()))
    .mockResolvedValueOnce(jsonResponse(emptyConfig()))
    .mockResolvedValueOnce(jsonResponse([]))
    // Preview
    .mockResolvedValueOnce(jsonResponse({ normalized: baseDoc(), changes: { fields: [{ field: "appearance", oldValue: "system", newValue: "dark" }] }, errors: [] }))
    // Apply
    .mockResolvedValueOnce(jsonResponse(baseDoc({ data: { appearance: "dark", interfaceDensity: "comfortable", terminalFontSize: 14, transcriptAutoScroll: true, defaultRightPanelTab: "review", destructiveConfirmations: true }, revision: 1 })))
    // Backups reload
    .mockResolvedValueOnce(jsonResponse([]))

  render(<SettingsPanel />)
  await waitFor(() => { expect(screen.getByTestId("prefs-appearance")).toBeInTheDocument() }, { timeout: 3000 })

  // Change appearance to dark.
  await userEvent.selectOptions(screen.getByTestId("prefs-appearance"), "dark")

  // Click preview.
  await userEvent.click(screen.getByTestId("settings-preview-button"))
  await waitFor(() => { expect(screen.getByTestId("preview-changes")).toBeInTheDocument() }, { timeout: 3000 })

  // Confirm apply.
  await userEvent.click(screen.getByTestId("confirm-apply"))
  await waitFor(() => {
    expect(screen.getByTestId("settings-message")).toHaveTextContent("Preferences applied.")
  }, { timeout: 3000 })
})
