export type PermissionRequest = {
  id: string
  sessionID: string
  permission: string
  patterns: string[]
  metadata: {
    command: string
  }
  always: string[]
  tool: {
    messageID: string
    callID: string
  }
}

export type PermissionReply = {
  reply: 'once' | 'always' | 'reject'
}
