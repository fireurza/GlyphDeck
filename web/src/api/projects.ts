import type { CreateProjectInput, Project } from '../types/project'
import { requestJson, requestDelete } from './client'

type ProjectsResponse = {
  projects: Project[]
}

export async function fetchProjects(signal?: AbortSignal) {
  const data = await requestJson<ProjectsResponse>('/api/projects', { signal })
  return data.projects
}

export async function createProject(input: CreateProjectInput) {
  return requestJson<Project>('/api/projects', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function deleteProject(projectId: string) {
  await requestDelete(`/api/projects/${encodeURIComponent(projectId)}`)
}
