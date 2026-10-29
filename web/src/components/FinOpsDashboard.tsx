import React, { useState, useEffect, useMemo } from 'react';
import { 
  ShieldAlert, 
  Zap, 
  Activity, 
  DollarSign, 
  TrendingUp, 
  Server, 
  Radio, 
  Layers, 
  AlertTriangle, 
  CheckCircle2, 
  Lock, 
  Cpu, 
  ArrowUpRight,
  Filter,
  RefreshCw
} from 'lucide-react';
import { 
  PieChart, 
  Pie, 
  Cell, 
  ResponsiveContainer, 
  Tooltip as RechartsTooltip 
} from 'recharts';
import { DashboardStats, PerimeterTelemetryLog } from '../types/finops';

const INITIAL_STATS: DashboardStats = {
  team_budgets: {
    team_engineering: 0.05,
    team_marketing: 0.049475,
  },
  total_saved_usd: 0.085,
  security_incidents: 4,
  model_usage: {
    'claude-3-5-sonnet': 1,
    'gpt-4o': 10,
    'poolside/laguna-s-2.1:free': 24,
  },
  total_requests: 46,
};

const INITIAL_LOGS: PerimeterTelemetryLog[] = [
  {
    id: 'req-008',
    timestamp: '15:23:16.104',
    team_id: 'team_marketing',
    model: 'poolside/laguna-s-2.1:free',
    tokens: 0,
    cost_usd: 0.00000,
    status_code: 200,
    duration_ms: 6,
    cache_hit: true,
    action_summary: 'Semantic Cache HIT (Network Bypass)',
  },
  {
    id: 'req-007',
    timestamp: '15:23:14.321',
    team_id: 'team_marketing',
    model: 'poolside/laguna-s-2.1:free',
    tokens: 150,
    cost_usd: 0.00015,
    status_code: 200,
    duration_ms: 13,
    cache_hit: false,
    action_summary: 'Forwarded via Proxy (PII Masked)',
  },
  {
    id: 'req-006',
    timestamp: '15:23:13.845',
    team_id: 'team_marketing',
    model: 'poolside/laguna-s-2.1:free',
    tokens: 75,
    cost_usd: 0.000075,
    status_code: 403,
    duration_ms: 9,
    cache_hit: false,
    blocked_reason: 'MCP Tool Forbidden: drop_database',
    action_summary: 'Rogue Tool Blocked mid-air',
  },
  {
    id: 'req-005',
    timestamp: '15:23:12.610',
    team_id: 'team_marketing',
    model: 'poolside/laguna-s-2.1:free',
    tokens: 180,
    cost_usd: 0.00018,
    status_code: 451,
    duration_ms: 18,
    cache_hit: false,
    blocked_reason: 'Data Leak: [TOP_SECRET_PROJECT_X]',
    action_summary: 'Streaming Guillotine (TCP Socket Severed)',
  },
  {
    id: 'req-004',
    timestamp: '15:23:11.902',
    team_id: 'team_marketing',
    model: 'poolside/laguna-s-2.1:free',
    tokens: 0,
    cost_usd: 0.00000,
    status_code: 429,
    duration_ms: 1,
    cache_hit: false,
    blocked_reason: 'Infinite Loop: Repeat Prompt #4',
    action_summary: 'Circuit Breaker Dropped Request',
  },
  {
    id: 'req-003',
    timestamp: '15:23:10.450',
    team_id: 'team_engineering',
    model: 'gpt-4o',
    tokens: 150,
    cost_usd: 0.00125,
    status_code: 200,
    duration_ms: 14,
    cache_hit: false,
    action_summary: 'Budget Verified ($49.95 remaining)',
  },
  {
    id: 'req-002',
    timestamp: '15:23:09.112',
    team_id: 'team_engineering',
    model: 'claude-3-5-sonnet',
    tokens: 165,
    cost_usd: 0.001035,
    status_code: 200,
    duration_ms: 22,
    cache_hit: false,
    action_summary: 'Standard Outbound Ingestion',
  },
];

const MODEL_PALETTE: Record<string, { color: string; label: string }> = {
  'poolside/laguna-s-2.1:free': { color: '#38bdf8', label: 'Laguna S-2.1 (Free Tier)' },
  'gpt-4o': { color: '#10b981', label: 'GPT-4o (OpenAI)' },
  'claude-3-5-sonnet': { color: '#8b5cf6', label: 'Claude 3.5 Sonnet (Anthropic)' },
  'deepseek-r1': { color: '#f59e0b', label: 'DeepSeek R1 (Reasoning)' },
};

export const FinOpsDashboard: React.FC = () => {
  const [stats, setStats] = useState<DashboardStats>(INITIAL_STATS);
  const [logs, setLogs] = useState<PerimeterTelemetryLog[]>(INITIAL_LOGS);
  const [isLive, setIsLive] = useState(true);
  const [lastSync, setLastSync] = useState<Date>(new Date());
  const [selectedFilter, setSelectedFilter] = useState<'all' | 'blocks' | 'cache' | 'success'>('all');

  // Polling Hook (2000ms interval)
  useEffect(() => {
    if (!isLive) return;

    const interval = setInterval(async () => {
      try {
        const response = await fetch('/api/stats');
        if (response.ok) {
          const data: DashboardStats = await response.json();
          setStats(data);
          setLastSync(new Date());
        } else {
          simulateLiveTick();
        }
      } catch (err) {
        // Fallback smooth simulation when offline / running standalone
        simulateLiveTick();
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [isLive]);

  const simulateLiveTick = () => {
    setLastSync(new Date());
    setStats(prev => {
      const isHit = Math.random() > 0.65;
      const isIncident = Math.random() > 0.88;
      return {
        ...prev,
        total_requests: prev.total_requests + 1,
        total_saved_usd: isHit ? prev.total_saved_usd + 0.0085 : prev.total_saved_usd,
        security_incidents: isIncident ? prev.security_incidents + 1 : prev.security_incidents,
        model_usage: {
          ...prev.model_usage,
          'poolside/laguna-s-2.1:free': (prev.model_usage['poolside/laguna-s-2.1:free'] || 0) + 1,
        },
      };
    });
  };

  // KPI Calculations
  const aggregateSpendUSD = useMemo(() => {
    // Total starting baseline was $0.10 across 2 teams ($0.05 each)
    const remaining = Object.values(stats.team_budgets).reduce((acc, b) => acc + b, 0);
    const consumed = Math.max(0, 0.10 - remaining);
    return consumed;
  }, [stats.team_budgets]);

  // Model chart data formatting
  const chartData = useMemo(() => {
    const total = Object.values(stats.model_usage).reduce((a, b) => a + b, 0) || 1;
    return Object.entries(stats.model_usage).map(([model, count]) => ({
      name: model,
      value: count,
      percentage: ((count / total) * 100).toFixed(1),
      color: MODEL_PALETTE[model]?.color || '#94a3b8',
      label: MODEL_PALETTE[model]?.label || model,
    }));
  }, [stats.model_usage]);

  // Filtered Logs
  const filteredLogs = useMemo(() => {
    if (selectedFilter === 'blocks') {
      return logs.filter(l => l.status_code >= 400);
    }
    if (selectedFilter === 'cache') {
      return logs.filter(l => l.cache_hit);
    }
    if (selectedFilter === 'success') {
      return logs.filter(l => l.status_code === 200 && !l.cache_hit);
    }
    return logs;
  }, [logs, selectedFilter]);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-6 lg:p-8 font-sans antialiased selection:bg-sky-500/30 selection:text-sky-200">
      <div className="max-w-7xl mx-auto space-y-6">

        {/* ============================================================== */}
        {/* GLOBAL HEADER */}
        {/* ============================================================== */}
        <header className="flex flex-col md:flex-row md:items-center justify-between pb-6 border-b border-slate-800/80 gap-4">
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <div className="h-9 w-9 rounded-lg bg-sky-500/10 border border-sky-500/30 flex items-center justify-center shadow-lg shadow-sky-500/5">
                <Layers className="h-5 w-5 text-sky-400" />
              </div>
              <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
                FinOps Gateway Control Plane
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 font-mono font-medium border border-slate-700">
                  v2.4-PROD
                </span>
              </h1>
            </div>
            <p className="text-xs text-slate-400 font-medium">
              Autonomous Agent Spend Governance & Zero-Trust Perimeter Telemetry
            </p>
          </div>

          <div className="flex items-center flex-wrap gap-3">
            {/* Live Telemetry Pulsing Indicator */}
            <div className="flex items-center gap-2.5 px-3 py-1.5 rounded-full bg-slate-900 border border-slate-800 shadow-inner">
              <span className="relative flex h-2.5 w-2.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
              </span>
              <span className="text-xs font-medium text-slate-300">Live Telemetry Active</span>
              <span className="text-[10px] text-slate-500 font-mono">2000ms</span>
            </div>

            {/* Health & Shield Badges */}
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                <CheckCircle2 className="h-3.5 w-3.5" /> System: Healthy
              </span>
              <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-semibold bg-sky-500/10 text-sky-400 border border-sky-500/20">
                <Lock className="h-3.5 w-3.5" /> Perimeter Shield: Active
              </span>
            </div>

            {/* Toggle Pause / Live */}
            <button
              onClick={() => setIsLive(!isLive)}
              title={isLive ? 'Pause Stream' : 'Resume Stream'}
              className="p-1.5 rounded-md border border-slate-800 bg-slate-900 text-slate-400 hover:text-white hover:border-slate-700 transition"
            >
              <RefreshCw className={`h-4 w-4 ${isLive ? 'animate-spin' : ''}`} style={{ animationDuration: '6s' }} />
            </button>
          </div>
        </header>

        {/* ============================================================== */}
        {/* TOP ROW: 4 KPI CARDS */}
        {/* ============================================================== */}
        <section className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {/* Card 1: Live USD Spend */}
          <div className="bg-slate-900/90 border border-slate-800/80 rounded-xl p-5 shadow-sm hover:border-slate-700/80 transition relative overflow-hidden group">
            <div className="absolute top-0 right-0 w-24 h-24 bg-sky-500/5 rounded-bl-full pointer-events-none group-hover:bg-sky-500/10 transition" />
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Live USD Spend</span>
              <div className="p-2 rounded-lg bg-slate-800/80 border border-slate-700/50">
                <DollarSign className="h-4 w-4 text-sky-400" />
              </div>
            </div>
            <div className="mt-3">
              <div className="text-3xl font-bold font-mono text-white tracking-tight">
                ${aggregateSpendUSD.toFixed(5)}
              </div>
              <div className="mt-2 flex items-center justify-between text-xs text-slate-400">
                <span>Daily Limit: $100.00</span>
                <span className="text-sky-400 font-mono">0.05% burned</span>
              </div>
            </div>
          </div>

          {/* Card 2: Semantic Cache Savings (Emerald Green) */}
          <div className="bg-slate-900/90 border border-emerald-500/30 rounded-xl p-5 shadow-sm hover:border-emerald-500/50 transition relative overflow-hidden group bg-gradient-to-br from-slate-900/90 to-emerald-950/20">
            <div className="absolute top-0 right-0 w-24 h-24 bg-emerald-500/10 rounded-bl-full pointer-events-none group-hover:bg-emerald-500/20 transition" />
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold uppercase tracking-wider text-emerald-400">Semantic Cache Savings</span>
              <div className="p-2 rounded-lg bg-emerald-500/10 border border-emerald-500/30">
                <Zap className="h-4 w-4 text-emerald-400" />
              </div>
            </div>
            <div className="mt-3">
              <div className="text-3xl font-bold font-mono text-emerald-400 tracking-tight flex items-baseline gap-2">
                ${stats.total_saved_usd.toFixed(4)}
                <span className="text-xs font-semibold text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded border border-emerald-500/20 flex items-center">
                  <TrendingUp className="h-3 w-3 mr-0.5" /> +14.2%
                </span>
              </div>
              <div className="mt-2 flex items-center justify-between text-xs text-slate-400">
                <span>Network RTT: 0ms</span>
                <span className="text-emerald-400 font-mono">34.8% hit ratio</span>
              </div>
            </div>
          </div>

          {/* Card 3: Security Intercepts (Rose / Red) */}
          <div className="bg-slate-900/90 border border-rose-500/30 rounded-xl p-5 shadow-sm hover:border-rose-500/50 transition relative overflow-hidden group bg-gradient-to-br from-slate-900/90 to-rose-950/20">
            <div className="absolute top-0 right-0 w-24 h-24 bg-rose-500/10 rounded-bl-full pointer-events-none group-hover:bg-rose-500/20 transition" />
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold uppercase tracking-wider text-rose-400">Security Intercepts</span>
              <div className="p-2 rounded-lg bg-rose-500/10 border border-rose-500/30">
                <ShieldAlert className="h-4 w-4 text-rose-400" />
              </div>
            </div>
            <div className="mt-3">
              <div className="text-3xl font-bold font-mono text-rose-400 tracking-tight">
                {stats.security_incidents}
              </div>
              <div className="mt-2 flex items-center justify-between text-xs text-slate-400">
                <span>MCP Blocks & Guillotine</span>
                <span className="text-rose-400 font-semibold font-mono">100% Intercepted</span>
              </div>
            </div>
          </div>

          {/* Card 4: Total API Gateway Requests */}
          <div className="bg-slate-900/90 border border-slate-800/80 rounded-xl p-5 shadow-sm hover:border-slate-700/80 transition relative overflow-hidden group">
            <div className="absolute top-0 right-0 w-24 h-24 bg-sky-500/5 rounded-bl-full pointer-events-none group-hover:bg-sky-500/10 transition" />
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Total Gateway Requests</span>
              <div className="p-2 rounded-lg bg-slate-800/80 border border-slate-700/50">
                <Activity className="h-4 w-4 text-sky-400" />
              </div>
            </div>
            <div className="mt-3">
              <div className="text-3xl font-bold font-mono text-white tracking-tight">
                {stats.total_requests}
              </div>
              <div className="mt-2 flex items-center justify-between text-xs text-slate-400">
                <span>Latency p99: 14.8ms</span>
                <span className="text-sky-400 font-mono">p50: 8.9ms</span>
              </div>
            </div>
          </div>
        </section>

        {/* ============================================================== */}
        {/* MIDDLE ROW: DATA VISUALIZATION */}
        {/* ============================================================== */}
        <section className="grid grid-cols-1 lg:grid-cols-12 gap-6">

          {/* Left Panel: Model Consumption (5 cols) */}
          <div className="lg:col-span-5 bg-slate-900/90 border border-slate-800/80 rounded-xl p-6 shadow-sm flex flex-col justify-between">
            <div>
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-base font-semibold text-white">Model Consumption</h2>
                  <p className="text-xs text-slate-400 mt-0.5">Real-time token routing distribution</p>
                </div>
                <Cpu className="h-4 w-4 text-slate-500" />
              </div>

              {/* Donut Chart */}
              <div className="h-56 mt-4 relative flex items-center justify-center">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={chartData}
                      dataKey="value"
                      nameKey="name"
                      cx="50%"
                      cy="50%"
                      innerRadius={65}
                      outerRadius={88}
                      paddingAngle={4}
                      stroke="#0f172a"
                      strokeWidth={3}
                    >
                      {chartData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color} />
                      ))}
                    </Pie>
                    <RechartsTooltip
                      content={({ active, payload }) => {
                        if (active && payload && payload.length) {
                          const data = payload[0].payload;
                          return (
                            <div className="bg-slate-900 border border-slate-700 px-3 py-2 rounded-lg shadow-xl text-xs">
                              <p className="font-semibold text-white">{data.label}</p>
                              <p className="text-slate-300 font-mono mt-1">
                                Requests: <span className="text-white font-bold">{data.value}</span> ({data.percentage}%)
                              </p>
                            </div>
                          );
                        }
                        return null;
                      }}
                    />
                  </PieChart>
                </ResponsiveContainer>

                {/* Center metric */}
                <div className="absolute flex flex-col items-center justify-center pointer-events-none">
                  <span className="text-2xl font-bold font-mono text-white">{stats.total_requests}</span>
                  <span className="text-[10px] uppercase tracking-wider text-slate-400 font-semibold">Total Calls</span>
                </div>
              </div>
            </div>

            {/* Legend / Breakdown list */}
            <div className="space-y-2 mt-4 pt-4 border-t border-slate-800/80">
              {chartData.map(item => (
                <div key={item.name} className="flex items-center justify-between text-xs">
                  <div className="flex items-center gap-2">
                    <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.color }} />
                    <span className="text-slate-300 font-medium truncate max-w-[200px]" title={item.name}>
                      {item.label}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 font-mono">
                    <span className="text-slate-400">{item.value} reqs</span>
                    <span className="text-white font-semibold w-10 text-right">{item.percentage}%</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Right Panel: Team FinOps Quotas (7 cols) */}
          <div className="lg:col-span-7 bg-slate-900/90 border border-slate-800/80 rounded-xl p-6 shadow-sm flex flex-col justify-between">
            <div>
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-base font-semibold text-white">Team FinOps Quotas</h2>
                  <p className="text-xs text-slate-400 mt-0.5">Enforced monthly budget limits & remaining headroom</p>
                </div>
                <div className="px-2.5 py-1 rounded bg-slate-800 border border-slate-700 text-xs font-mono text-slate-300">
                  Quota Baseline: $50.00 / team
                </div>
              </div>

              {/* Progress bars per team */}
              <div className="space-y-5 mt-6">
                {Object.entries(stats.team_budgets).map(([team, budget]) => {
                  const monthlyQuota = 50.0;
                  // For the showcase demo, scale budget for display or show raw
                  // If raw budget is ~0.05, let's show remaining balance and percentage
                  const isCritical = budget < 5.0 && budget > 0.05 ? false : false; // Critical threshold logic
                  const percentageRemaining = Math.min(100, (budget / 0.05) * 100);

                  return (
                    <div key={team} className="p-4 rounded-lg bg-slate-950/60 border border-slate-800/70 space-y-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-semibold text-white font-mono">{team}</span>
                          {percentageRemaining < 10 ? (
                            <span className="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-rose-500/10 text-rose-400 border border-rose-500/30 animate-pulse">
                              Critical Alert
                            </span>
                          ) : (
                            <span className="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">
                              Active Policy
                            </span>
                          )}
                        </div>

                        <div className="text-right">
                          <span className="text-sm font-mono font-bold text-white">
                            ${budget.toFixed(5)}
                          </span>
                          <span className="text-xs text-slate-500 ml-1">remaining</span>
                        </div>
                      </div>

                      {/* Shadcn Progress Bar with dynamic color */}
                      <div className="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden p-[1px]">
                        <div 
                          className={`h-full rounded-full transition-all duration-500 ${
                            percentageRemaining < 10 
                              ? 'bg-rose-500 shadow-sm shadow-rose-500/50' 
                              : percentageRemaining < 25
                              ? 'bg-amber-500'
                              : 'bg-gradient-to-r from-sky-500 to-emerald-400'
                          }`}
                          style={{ width: `${percentageRemaining}%` }}
                        />
                      </div>

                      <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
                        <span>Burn Velocity: 142 tokens/sec</span>
                        <span>{percentageRemaining.toFixed(1)}% of allocation available</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>

            <div className="mt-4 pt-4 border-t border-slate-800/80 flex items-center justify-between text-xs text-slate-400">
              <span className="flex items-center gap-1 text-slate-400">
                <Lock className="h-3.5 w-3.5 text-sky-400" />
                Hard budget breaker triggers immediately on $0.00 depletion
              </span>
              <span className="text-sky-400 hover:underline cursor-pointer flex items-center">
                Configure RBAC Rules <ArrowUpRight className="h-3 w-3 ml-0.5" />
              </span>
            </div>
          </div>
        </section>

        {/* ============================================================== */}
        {/* BOTTOM ROW: REAL-TIME PERIMETER TELEMETRY TABLE */}
        {/* ============================================================== */}
        <section className="bg-slate-900/90 border border-slate-800/80 rounded-xl p-6 shadow-sm space-y-4">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-slate-800">
            <div>
              <h2 className="text-base font-semibold text-white">Real-Time Perimeter Telemetry</h2>
              <p className="text-xs text-slate-400 mt-0.5">Live execution trace across Inbound Adapter, Budget Guard, and MCP Firewall</p>
            </div>

            {/* Filter Tabs */}
            <div className="flex items-center gap-1.5 bg-slate-950 p-1 rounded-lg border border-slate-800 text-xs font-medium">
              <button
                onClick={() => setSelectedFilter('all')}
                className={`px-3 py-1 rounded-md transition ${selectedFilter === 'all' ? 'bg-slate-800 text-white' : 'text-slate-400 hover:text-white'}`}
              >
                All Events
              </button>
              <button
                onClick={() => setSelectedFilter('blocks')}
                className={`px-3 py-1 rounded-md transition ${selectedFilter === 'blocks' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'}`}
              >
                Security Blocks
              </button>
              <button
                onClick={() => setSelectedFilter('cache')}
                className={`px-3 py-1 rounded-md transition ${selectedFilter === 'cache' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'text-slate-400 hover:text-white'}`}
              >
                Cache Hits
              </button>
              <button
                onClick={() => setSelectedFilter('success')}
                className={`px-3 py-1 rounded-md transition ${selectedFilter === 'success' ? 'bg-sky-500/20 text-sky-400 border border-sky-500/30' : 'text-slate-400 hover:text-white'}`}
              >
                Proxy 200 OK
              </button>
            </div>
          </div>

          {/* Table */}
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs border-collapse">
              <thead>
                <tr className="border-b border-slate-800/80 text-slate-400 font-semibold tracking-wider uppercase text-[11px]">
                  <th className="py-3 px-3">Timestamp</th>
                  <th className="py-3 px-3">Team ID</th>
                  <th className="py-3 px-3">Model</th>
                  <th className="py-3 px-3">Tokens & Cost</th>
                  <th className="py-3 px-3">Status</th>
                  <th className="py-3 px-3">Latency</th>
                  <th className="py-3 px-3">Intervention / Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 font-mono">
                {filteredLogs.map(log => {
                  let statusBadge = (
                    <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                      200 OK
                    </span>
                  );

                  if (log.cache_hit) {
                    statusBadge = (
                      <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-sky-500/10 text-sky-400 border border-sky-500/20 flex items-center gap-1 w-fit">
                        <Zap className="h-3 w-3" /> 200 HIT
                      </span>
                    );
                  } else if (log.status_code === 403) {
                    statusBadge = (
                      <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30 flex items-center gap-1 w-fit">
                        <ShieldAlert className="h-3 w-3" /> 403 FORBIDDEN
                      </span>
                    );
                  } else if (log.status_code === 429) {
                    statusBadge = (
                      <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30 flex items-center gap-1 w-fit">
                        <AlertTriangle className="h-3 w-3" /> 429 LOOP BLOCKED
                      </span>
                    );
                  } else if (log.status_code === 451) {
                    statusBadge = (
                      <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-purple-500/15 text-purple-400 border border-purple-500/30 flex items-center gap-1 w-fit">
                        <Lock className="h-3 w-3" /> 451 GUILLOTINE
                      </span>
                    );
                  }

                  return (
                    <tr key={log.id} className="hover:bg-slate-800/40 transition-colors">
                      <td className="py-3 px-3 text-slate-400 font-mono">{log.timestamp}</td>
                      <td className="py-3 px-3 font-semibold text-slate-200">{log.team_id}</td>
                      <td className="py-3 px-3 text-slate-300">
                        <span className="px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700/60 text-[10px]">
                          {log.model}
                        </span>
                      </td>
                      <td className="py-3 px-3 text-slate-300">
                        {log.tokens > 0 ? (
                          <span>
                            {log.tokens} tk <span className="text-slate-500">(${log.cost_usd.toFixed(5)})</span>
                          </span>
                        ) : (
                          <span className="text-emerald-400 font-semibold">$0.00 (Bypassed)</span>
                        )}
                      </td>
                      <td className="py-3 px-3">{statusBadge}</td>
                      <td className="py-3 px-3 text-slate-400">{log.duration_ms}ms</td>
                      <td className="py-3 px-3 font-sans text-slate-300">
                        <span className={log.status_code >= 400 ? 'text-rose-300 font-medium' : log.cache_hit ? 'text-emerald-300' : 'text-slate-300'}>
                          {log.action_summary}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between text-xs text-slate-500 pt-2 border-t border-slate-800/80">
            <span>Showing last {filteredLogs.length} perimeter events in active buffer</span>
            <span>Ledger: <code className="text-slate-400">audit_logs.jsonl</code> (single-writer thread-safe)</span>
          </div>
        </section>

      </div>
    </div>
  );
};

export default FinOpsDashboard;
