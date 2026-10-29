export interface DashboardStats {
  team_budgets: Record<string, number>;
  total_saved_usd: number;
  security_incidents: number;
  model_usage: Record<string, number>;
  total_requests: number;
}

export interface PerimeterTelemetryLog {
  id: string;
  timestamp: string;
  team_id: string;
  model: string;
  tokens: number;
  cost_usd: number;
  status_code: number;
  duration_ms: number;
  cache_hit: boolean;
  blocked_reason?: string;
  action_summary: string;
}
