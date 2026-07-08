export type GlyphSession = {
  id: string
  title: string
  projectID?: string
  directory?: string
  agent?: string
}

export type GlyphMessage = {
  info: {
    id: string
    role: string
  }
  parts: GlyphPart[]
}

export type GlyphPart = {
  type: string
  text?: string
  tool?: string
  [key: string]: unknown
}

export type PromptResult = {
  messageID: string
  role: string
  text: string
  parts: GlyphPart[]
}

export type CreateSessionRequest = {
  title: string
}

export type SendPromptRequest = {
  text: string
}
