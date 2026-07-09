export type ReviewResponse = {
  project: ProjectSummary
  git: GitSummary
  session: SessionSummary
  activity: ActivitySummary
  updatedAt: string
}

export type ProjectSummary = {
  id: string
  name: string
  path: string
  trusted: boolean
}

export type GitSummary = {
  available: boolean
  branch: string
  dirty: boolean
  changedFiles: string[]
}

export type SessionSummary = {
  id: string
  messageCount: number
  lastAssistantSummary: string
}

export type ActivitySummary = {
  messageCount: number
  toolEvents: number
  note?: string
}
