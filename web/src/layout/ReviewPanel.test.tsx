import { render, screen } from '@testing-library/react'
import ReviewPanel from './ReviewPanel'
import { fetchReview } from '../api/review'
import type { ReviewResponse } from '../types/review'

vi.mock('../api/review', () => ({
  fetchReview: vi.fn(),
}))

test('renders review data when changed files is null at runtime', async () => {
  vi.mocked(fetchReview).mockResolvedValue({
    project: {
      id: 'proj-1',
      name: 'Validation Project',
      path: 'C:\\Validation',
      trusted: true,
    },
    git: {
      available: true,
      branch: 'main',
      dirty: false,
      changedFiles: null,
    },
    session: {
      id: 'ses-1',
      messageCount: 0,
      lastAssistantSummary: '',
    },
    activity: {
      messageCount: 0,
      toolEvents: 0,
    },
    updatedAt: '2026-01-01T00:00:00Z',
  } as unknown as ReviewResponse)

  render(<ReviewPanel selectedProjectId="proj-1" selectedSessionId="ses-1" />)

  expect(await screen.findByTestId('review-project-name')).toHaveTextContent(
    'Validation Project',
  )
  expect(screen.getByTestId('review-git-status')).toHaveTextContent('Clean')
  expect(screen.queryByTestId('review-changed-files')).not.toBeInTheDocument()
})
