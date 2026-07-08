export type OpencodeStatus = {
  installed: boolean
  executable?: string
  version?: string
  status: string
}

export type ServerStatus = {
  projectId?: string
  status: string
  baseUrl?: string
  port?: number
  pid?: number
  version?: string
}
