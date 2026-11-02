// Состояние пинга подключения: ms = -1 означает ошибку.
export type PingState = { ms: number, error?: string } | 'pending'
