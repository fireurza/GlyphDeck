export type UsageResponse = {
  providerID: string
  modelID: string
  agent: string
  mode: string
  cost: number
  tokens: TokenUsage
  messageCount: number
  updatedAt?: string
}

export type TokenUsage = {
  total: number
  input: number
  output: number
  reasoning: number
  cache: CacheUsage
}

export type CacheUsage = {
  read: number
  write: number
}
