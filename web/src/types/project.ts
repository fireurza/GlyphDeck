export type Project = {
  id: string
  name: string
  path: string
  trusted: boolean
  tags: string[]
  git: {
    isRepo: boolean
    branch: string
  }
}

export type CreateProjectInput = {
  name: string
  path: string
  trusted: boolean
}
