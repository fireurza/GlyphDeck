import '@testing-library/jest-dom/vitest'

beforeEach(() => {
  vi.restoreAllMocks()
})

if (typeof HTMLDialogElement !== 'undefined') {
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.open = true
    this.setAttribute('open', '')
  }

  HTMLDialogElement.prototype.close = function close() {
    this.open = false
    this.removeAttribute('open')
  }
}
