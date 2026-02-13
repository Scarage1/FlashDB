'use client';

import { useState, useCallback } from 'react';
import { motion } from 'framer-motion';
import { Gauge, Play, RotateCcw, Zap, Clock, Activity, Timer, Cpu, ArrowUpRight, Layers } from 'lucide-react';
import { runBenchmark, type BenchmarkResult } from '@/lib/api';
import PageHeader from '@/components/ui/PageHeader';
import StatCard from '@/components/ui/StatCard';
import Badge from '@/components/ui/Badge';
import EmptyState from '@/components/ui/EmptyState';
import { SkeletonCard } from '@/components/ui/Skeleton';

const container = { hidden: {}, show: { transition: { staggerChildren: 0.06 } } };
const item = { hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0 } };

function fmtLatency(ns: number) { if (ns >= 1e9) return `${(ns / 1e9).toFixed(2)}s`; if (ns >= 1e6) return `${(ns / 1e6).toFixed(2)}ms`; if (ns >= 1e3) return `${(ns / 1e3).toFixed(1)}μs`; return `${ns}ns`; }
function fmtDuration(ns: number) { if (ns >= 1e9) return `${(ns / 1e9).toFixed(2)}s`; if (ns >= 1e6) return `${(ns / 1e6).toFixed(1)}ms`; return `${(ns / 1e3).toFixed(0)}μs`; }
function fmtOps(n: number) { if (n >= 1e6) return `${(n / 1e6).toFixed(2)}M`; if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`; return String(Math.round(n)); }

const PRESETS = [
  { label: '1K', ops: 1000 },
  { label: '5K', ops: 5000 },
  { label: '10K', ops: 10000 },
  { label: '50K', ops: 50000 },
  { label: '100K', ops: 100000 },
];

function ThroughputBar({ label, value, max, color }: { label: string; value: number; max: number; color: string }) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <span className="text-xs font-semibold" style={{ color: 'var(--text-secondary)' }}>{label}</span>
        <span className="text-xs font-mono font-bold" style={{ color }}>{fmtOps(value)}/s</span>
      </div>
      <div className="h-3 rounded-full overflow-hidden" style={{ background: 'var(--bg-inset)' }}>
        <motion.div className="h-full rounded-full" initial={{ width: 0 }}
          animate={{ width: `${Math.min(100, (value / max) * 100)}%` }}
          transition={{ duration: 0.8, ease: 'easeOut' }}
          style={{ background: color }} />
      </div>
    </div>
  );
}

export default function BenchmarkPage() {
  const [operations, setOperations] = useState(10000);
  const [result, setResult] = useState<BenchmarkResult | null>(null);
  const [history, setHistory] = useState<BenchmarkResult[]>([]);
  const [running, setRunning] = useState(false);

  const run = useCallback(async () => {
    setRunning(true);
    const r = await runBenchmark(operations);
    setResult(r);
    if (r) setHistory((h) => [...h, r].slice(-10));
    setRunning(false);
  }, [operations]);

  const p99Color = result ? (result.p99_latency_ns < 1e3 ? '#22c55e' : result.p99_latency_ns < 1e5 ? '#f59e0b' : '#ef4444') : '#22c55e';
  const throughputMax = result ? Math.max(result.set_ops_per_sec, result.get_ops_per_sec, result.del_ops_per_sec, result.concurrent_ops_per_sec || 0) * 1.15 : 1;

  return (
    <motion.div className="max-w-5xl mx-auto px-4 sm:px-6 py-8" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.35 }}>
      <PageHeader title="Benchmark" description="Comprehensive throughput, latency & concurrency analysis"
        badge={<Badge variant="soft" color="#f59e0b"><Gauge size={12} /> Performance</Badge>} />

      {/* Controls */}
      <div className="card p-5 mb-6">
        <div className="flex flex-wrap items-end gap-5">
          <div className="flex-1 min-w-[200px]">
            <label className="text-xs font-semibold mb-2 block" style={{ color: 'var(--text-secondary)' }}>Operations (per phase)</label>
            <div className="flex items-center gap-2">
              <input type="number" value={operations} onChange={(e) => setOperations(Number(e.target.value) || 1000)} className="input-field !text-sm font-mono flex-1" min={100} step={1000} />
            </div>
            <div className="flex gap-1.5 mt-2.5">
              {PRESETS.map((p) => (
                <button key={p.label} onClick={() => setOperations(p.ops)}
                  className="px-2.5 py-1 rounded-lg text-xs font-mono transition-colors"
                  style={{ background: operations === p.ops ? 'var(--brand-muted)' : 'var(--bg-secondary)', color: operations === p.ops ? 'var(--brand)' : 'var(--text-tertiary)' }}>
                  {p.label}
                </button>
              ))}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={run} disabled={running} className="btn-primary !text-sm disabled:opacity-50">
              {running ? <><RotateCcw size={16} className="animate-spin" /> Running…</> : <><Play size={16} /> Run Benchmark</>}
            </button>
          </div>
        </div>
      </div>

      {/* Results */}
      {running ? (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">{Array.from({ length: 8 }).map((_, i) => <SkeletonCard key={i} />)}</div>
      ) : !result ? (
        <div className="card mb-6"><EmptyState icon={<Gauge size={28} />} title="Ready to benchmark" description="Runs 5 phases: SET → GET → Mixed → Concurrent → DEL with per-op metrics and latency percentiles." /></div>
      ) : (
        <>
          {/* Primary KPIs */}
          <motion.div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-4" variants={container} initial="hidden" animate="show">
            <motion.div variants={item}><StatCard icon={<Zap size={18} />} label="Mixed Ops/sec" value={fmtOps(result.ops_per_sec)} color="#f59e0b" /></motion.div>
            <motion.div variants={item}><StatCard icon={<Clock size={18} />} label="P50 Latency" value={fmtLatency(result.p50_latency_ns)} color="#22c55e" /></motion.div>
            <motion.div variants={item}><StatCard icon={<Clock size={18} />} label="P99 Latency" value={fmtLatency(result.p99_latency_ns)} color={p99Color} /></motion.div>
            <motion.div variants={item}><StatCard icon={<Timer size={18} />} label="Duration" value={fmtDuration(result.duration_ns)} color="#0284c7" /></motion.div>
          </motion.div>

          {/* Secondary KPIs */}
          <motion.div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6" variants={container} initial="hidden" animate="show">
            <motion.div variants={item}><StatCard icon={<ArrowUpRight size={18} />} label="SET Ops/sec" value={fmtOps(result.set_ops_per_sec)} color="#22c55e" /></motion.div>
            <motion.div variants={item}><StatCard icon={<Activity size={18} />} label="GET Ops/sec" value={fmtOps(result.get_ops_per_sec)} color="#0284c7" /></motion.div>
            <motion.div variants={item}><StatCard icon={<Cpu size={18} />} label={`Concurrent (${result.concurrency}T)`} value={result.concurrent_ops_per_sec ? fmtOps(result.concurrent_ops_per_sec) : '—'} color="#8b5cf6" /></motion.div>
            <motion.div variants={item}><StatCard icon={<Layers size={18} />} label="Scale Factor" value={result.scale_factor ? `${result.scale_factor}×` : '—'} color={result.scale_factor && result.scale_factor > 1.5 ? '#22c55e' : '#f59e0b'} /></motion.div>
          </motion.div>
        </>
      )}

      {/* Throughput Bars */}
      {result && (
        <motion.div className="card p-5 mb-6 space-y-4" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }}>
          <div className="flex items-center justify-between mb-1">
            <span className="text-sm font-display font-bold" style={{ color: 'var(--text-primary)' }}>Throughput Breakdown</span>
            <span className="text-xs font-mono" style={{ color: 'var(--text-tertiary)' }}>ops/sec</span>
          </div>
          <ThroughputBar label="SET" value={result.set_ops_per_sec} max={throughputMax} color="#22c55e" />
          <ThroughputBar label="GET" value={result.get_ops_per_sec} max={throughputMax} color="#0284c7" />
          <ThroughputBar label="DEL" value={result.del_ops_per_sec} max={throughputMax} color="#ef4444" />
          <ThroughputBar label="Mixed (SET+GET)" value={result.ops_per_sec} max={throughputMax} color="#f59e0b" />
          {result.concurrent_ops_per_sec ? <ThroughputBar label={`Concurrent (${result.concurrency} threads)`} value={result.concurrent_ops_per_sec} max={throughputMax} color="#8b5cf6" /> : null}
        </motion.div>
      )}

      {/* Latency Distribution */}
      {result && (
        <motion.div className="card overflow-hidden mb-6" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }}>
          <div className="flex items-center gap-2.5 px-5 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
            <Clock size={16} style={{ color: 'var(--brand)' }} />
            <h3 className="text-sm font-display font-bold" style={{ color: 'var(--text-primary)' }}>Latency Distribution</h3>
          </div>
          <div className="grid grid-cols-4 divide-x" style={{ borderColor: 'var(--border)' }}>
            {[
              { label: 'Average', value: result.avg_latency_ns, color: '#71717a' },
              { label: 'P50', value: result.p50_latency_ns, color: '#22c55e' },
              { label: 'P99', value: result.p99_latency_ns, color: '#f59e0b' },
              { label: 'P99.9', value: result.p999_latency_ns, color: '#ef4444' },
            ].map((p) => (
              <div key={p.label} className="text-center py-4 px-3">
                <div className="text-xs font-semibold mb-1" style={{ color: 'var(--text-tertiary)' }}>{p.label}</div>
                <div className="text-lg font-mono font-bold" style={{ color: p.color }}>{fmtLatency(p.value)}</div>
              </div>
            ))}
          </div>
        </motion.div>
      )}

      {/* History */}
      {history.length > 1 && (
        <div className="card overflow-hidden">
          <div className="flex items-center gap-2.5 px-5 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
            <Activity size={16} style={{ color: 'var(--brand)' }} />
            <h3 className="text-sm font-display font-bold" style={{ color: 'var(--text-primary)' }}>Run History</h3>
            <Badge variant="outline" color="var(--text-tertiary)">{history.length} runs</Badge>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr style={{ background: 'var(--bg-secondary)' }}>
                  <th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>#</th>
                  <th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>SET</th>
                  <th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>GET</th>
                  <th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Mixed</th>
                  <th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Concurrent</th>
                  <th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>P99</th>
                  <th className="text-left py-2.5 px-4 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Scale</th>
                </tr>
              </thead>
              <tbody>
                {[...history].reverse().map((r, i) => (
                  <tr key={i} style={{ borderTop: '1px solid var(--border)' }}
                    onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-secondary)'}
                    onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
                    <td className="py-2.5 px-4 text-xs font-mono" style={{ color: 'var(--text-tertiary)' }}>#{history.length - i}</td>
                    <td className="py-2.5 px-4 text-xs font-mono" style={{ color: '#22c55e' }}>{fmtOps(r.set_ops_per_sec)}</td>
                    <td className="py-2.5 px-4 text-xs font-mono" style={{ color: '#0284c7' }}>{fmtOps(r.get_ops_per_sec)}</td>
                    <td className="py-2.5 px-4 text-xs font-mono font-bold" style={{ color: 'var(--brand)' }}>{fmtOps(r.ops_per_sec)}</td>
                    <td className="py-2.5 px-4 text-xs font-mono" style={{ color: '#8b5cf6' }}>{r.concurrent_ops_per_sec ? fmtOps(r.concurrent_ops_per_sec) : '—'}</td>
                    <td className="py-2.5 px-4 text-xs font-mono" style={{ color: r.p99_latency_ns < 1e3 ? '#22c55e' : '#f59e0b' }}>{fmtLatency(r.p99_latency_ns)}</td>
                    <td className="py-2.5 px-4 text-xs font-mono font-bold" style={{ color: r.scale_factor && r.scale_factor > 1.5 ? '#22c55e' : '#f59e0b' }}>{r.scale_factor ? `${r.scale_factor}×` : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </motion.div>
  );
}
