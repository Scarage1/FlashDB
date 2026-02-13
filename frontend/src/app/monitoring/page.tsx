'use client';

import { useState, useEffect, useCallback } from 'react';
import { motion } from 'framer-motion';
import { Activity, Clock, HardDrive, MemoryStick, Server, RefreshCw, Zap, AlertTriangle } from 'lucide-react';
import { executeCommand } from '@/lib/api';
import PageHeader from '@/components/ui/PageHeader';
import StatCard from '@/components/ui/StatCard';
import Badge from '@/components/ui/Badge';
import { SkeletonCard, SkeletonTable } from '@/components/ui/Skeleton';
import EmptyState from '@/components/ui/EmptyState';

interface SlowEntry { id: number; timestamp: number; duration: number; command: string; }

const container = { hidden: {}, show: { transition: { staggerChildren: 0.06 } } };
const item = { hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0 } };

function fmtUptime(s: number) {
  const d = Math.floor(s / 86400); const h = Math.floor((s % 86400) / 3600); const m = Math.floor((s % 3600) / 60);
  if (d) return `${d}d ${h}h ${m}m`; if (h) return `${h}h ${m}m`; return `${m}m`;
}
function fmtMem(b: string | number) { const n = Number(b); if (n >= 1e9) return `${(n / 1e9).toFixed(1)} GB`; if (n >= 1e6) return `${(n / 1e6).toFixed(1)} MB`; if (n >= 1e3) return `${(n / 1e3).toFixed(1)} KB`; return `${n} B`; }

export default function MonitoringPage() {
  const [info, setInfo] = useState<Record<string, string>>({});
  const [slowLog, setSlowLog] = useState<SlowEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const fetchAll = useCallback(async () => {
    const infoRes = await executeCommand('INFO');
    const map: Record<string, string> = {};
    const raw = typeof infoRes.result === 'string' ? infoRes.result : '';
    raw.split('\n').forEach((line: string) => {
      if (!line || line.startsWith('#')) return;
      const idx = line.indexOf(':');
      if (idx > 0) map[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
    });
    setInfo(map);

    const slowRes = await executeCommand('SLOWLOG GET 25');
    if (Array.isArray(slowRes.result)) {
      const entries: SlowEntry[] = slowRes.result.map((e: unknown, i: number) => {
        if (Array.isArray(e)) return { id: Number(e[0] ?? i), timestamp: Number(e[1] ?? 0), duration: Number(e[2] ?? 0), command: Array.isArray(e[3]) ? e[3].join(' ') : String(e[3] ?? '') };
        return { id: i, timestamp: 0, duration: 0, command: String(e ?? '') };
      });
      setSlowLog(entries);
    }
    setLoading(false);
  }, []);

  useEffect(() => { fetchAll(); const iv = setInterval(fetchAll, 8000); return () => clearInterval(iv); }, [fetchAll]);
  const refresh = async () => { setRefreshing(true); await fetchAll(); setRefreshing(false); };

  const kpis = [
    { icon: <Clock size={18} />, label: 'Uptime', value: fmtUptime(Number(info.uptime_in_seconds || 0)), color: '#f59e0b' },
    { icon: <MemoryStick size={18} />, label: 'Memory Used', value: fmtMem(info.used_memory || '0'), color: '#0284c7' },
    { icon: <HardDrive size={18} />, label: 'DB Keys', value: info.keys || '0', color: '#8b5cf6' },
    { icon: <Activity size={18} />, label: 'Total Reads', value: Number(info.total_reads || 0).toLocaleString(), color: '#059669' },
    { icon: <Zap size={18} />, label: 'Total Writes', value: Number(info.total_writes || 0).toLocaleString(), color: '#ef4444' },
    { icon: <Server size={18} />, label: 'Version', value: info.flashdb_version || '—', color: '#71717a' },
  ];

  return (
    <motion.div className="max-w-6xl mx-auto px-4 sm:px-6 py-8" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.35 }}>
      <PageHeader title="Monitoring" description="Real-time server metrics · refreshes every 8s"
        badge={<Badge variant="soft" color="#22c55e">● Live</Badge>}
        actions={<button onClick={refresh} className="btn-secondary !px-3 !py-1.5 !text-xs"><RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} /> Refresh</button>} />

      {/* KPIs */}
      {loading ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 mb-8">{Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}</div>
      ) : (
        <motion.div className="grid grid-cols-2 sm:grid-cols-3 gap-4 mb-8" variants={container} initial="hidden" animate="show">
          {kpis.map((k) => (
            <motion.div key={k.label} variants={item}><StatCard icon={k.icon} label={k.label} value={k.value} color={k.color} /></motion.div>
          ))}
        </motion.div>
      )}

      {/* Slow Queries */}
      <div className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, var(--error) 10%, transparent)' }}>
              <AlertTriangle size={16} style={{ color: 'var(--error)' }} />
            </div>
            <div>
              <h3 className="font-display text-sm font-bold" style={{ color: 'var(--text-primary)' }}>Slow Queries</h3>
              <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{slowLog.length} entries</p>
            </div>
          </div>
          <Badge variant="outline" color="#f59e0b">SLOWLOG</Badge>
        </div>

        {loading ? <SkeletonTable rows={5} /> : !slowLog.length ? (
          <EmptyState icon={<Activity size={24} />} title="No slow queries" description="No commands exceeded the threshold. Your database is performing well." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr style={{ background: 'var(--bg-secondary)' }}>
                  <th className="text-left py-2.5 px-5 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>ID</th>
                  <th className="text-left py-2.5 px-5 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Duration</th>
                  <th className="text-left py-2.5 px-5 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Command</th>
                  <th className="text-left py-2.5 px-5 text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>Timestamp</th>
                </tr>
              </thead>
              <tbody>
                {slowLog.map((entry) => (
                  <tr key={entry.id} className="transition-colors" style={{ borderTop: '1px solid var(--border)' }}
                    onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-secondary)'}
                    onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
                    <td className="py-2.5 px-5 text-xs font-mono" style={{ color: 'var(--text-tertiary)' }}>#{entry.id}</td>
                    <td className="py-2.5 px-5 text-xs font-mono font-bold" style={{ color: entry.duration > 10000 ? 'var(--error)' : 'var(--warning)' }}>{entry.duration}μs</td>
                    <td className="py-2.5 px-5 font-mono text-xs" style={{ color: 'var(--text-primary)' }}>{entry.command}</td>
                    <td className="py-2.5 px-5 text-xs" style={{ color: 'var(--text-tertiary)' }}>{entry.timestamp ? new Date(entry.timestamp * 1000).toLocaleTimeString() : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </motion.div>
  );
}
