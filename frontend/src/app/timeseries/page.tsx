'use client';

import { useState, useCallback } from 'react';
import { motion } from 'framer-motion';
import { LineChart, Plus, RefreshCw, Info, Clock, Database, HardDrive, Timer, Search } from 'lucide-react';
import { tsAdd, tsRange, tsInfo, type TSDataPoint, type TSInfo } from '@/lib/api';
import { useToast } from '@/context/ToastContext';
import PageHeader from '@/components/ui/PageHeader';
import StatCard from '@/components/ui/StatCard';
import Badge from '@/components/ui/Badge';
import EmptyState from '@/components/ui/EmptyState';
import { SkeletonCard, SkeletonChart } from '@/components/ui/Skeleton';

const container = { hidden: {}, show: { transition: { staggerChildren: 0.06 } } };
const item = { hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0 } };

function fmtTs(ms: number) { return ms ? new Date(ms).toLocaleTimeString() : '—'; }
function fmtMem(b: number) { if (b >= 1e6) return `${(b / 1e6).toFixed(1)} MB`; if (b >= 1e3) return `${(b / 1e3).toFixed(1)} KB`; return `${b} B`; }

export default function TimeSeriesPage() {
  const { showToast } = useToast();
  const [seriesKey, setSeriesKey] = useState('');
  const [addKey, setAddKey] = useState('');
  const [addValue, setAddValue] = useState('');
  const [queryKey, setQueryKey] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [dataPoints, setDataPoints] = useState<TSDataPoint[]>([]);
  const [seriesInfo, setSeriesInfo] = useState<TSInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [infoLoading, setInfoLoading] = useState(false);

  const handleAdd = useCallback(async () => {
    if (!addKey.trim() || !addValue.trim()) return;
    await tsAdd(addKey.trim(), Number(addValue));
    showToast(`Added data point to ${addKey.trim()}`, 'success');
    setAddValue('');
  }, [addKey, addValue, showToast]);

  const handleQuery = useCallback(async () => {
    if (!queryKey.trim()) return;
    setLoading(true);
    const fromMs = from ? new Date(from).getTime() : Date.now() - 3600000;
    const toMs = to ? new Date(to).getTime() : Date.now();
    const pts = await tsRange(queryKey.trim(), fromMs, toMs);
    setDataPoints(pts);
    setLoading(false);
  }, [queryKey, from, to]);

  const handleInfo = useCallback(async () => {
    if (!seriesKey.trim()) return;
    setInfoLoading(true);
    const info = await tsInfo(seriesKey.trim());
    setSeriesInfo(info);
    setInfoLoading(false);
  }, [seriesKey]);

  const maxVal = Math.max(1, ...dataPoints.map((d) => d.val));
  const minVal = Math.min(0, ...dataPoints.map((d) => d.val));
  const range = maxVal - minVal || 1;

  return (
    <motion.div className="max-w-6xl mx-auto px-4 sm:px-6 py-8" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.35 }}>
      <PageHeader title="Time Series" description="Add, query, and visualize time-series data"
        badge={<Badge variant="soft" color="#0284c7"><LineChart size={12} /> TS Engine</Badge>} />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Add + Query controls */}
        <div className="space-y-5">
          {/* Add Data Point */}
          <div className="card p-5 space-y-4">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, #22c55e 10%, transparent)' }}>
                <Plus size={16} style={{ color: '#22c55e' }} />
              </div>
              <h3 className="font-display text-sm font-bold" style={{ color: 'var(--text-primary)' }}>Add Data Point</h3>
            </div>
            <div className="space-y-3">
              <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>Series Key</label>
                <input value={addKey} onChange={(e) => setAddKey(e.target.value)} className="input-field !text-xs" placeholder="sensor:temperature" /></div>
              <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>Value</label>
                <input value={addValue} onChange={(e) => setAddValue(e.target.value)} className="input-field !text-xs" placeholder="42.5" type="number"
                  onKeyDown={(e) => e.key === 'Enter' && handleAdd()} /></div>
            </div>
            <button onClick={handleAdd} disabled={!addKey.trim() || !addValue.trim()} className="btn-primary w-full !text-xs disabled:opacity-40">
              <Plus size={14} /> Add Point
            </button>
          </div>

          {/* Query Range */}
          <div className="card p-5 space-y-4">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, var(--brand) 10%, transparent)' }}>
                <Search size={16} style={{ color: 'var(--brand)' }} />
              </div>
              <h3 className="font-display text-sm font-bold" style={{ color: 'var(--text-primary)' }}>Query Range</h3>
            </div>
            <div className="space-y-3">
              <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>Series Key</label>
                <input value={queryKey} onChange={(e) => setQueryKey(e.target.value)} className="input-field !text-xs" placeholder="sensor:temperature" /></div>
              <div className="grid grid-cols-2 gap-3">
                <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>From</label>
                  <input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} className="input-field !text-xs" /></div>
                <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>To</label>
                  <input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} className="input-field !text-xs" /></div>
              </div>
            </div>
            <button onClick={handleQuery} disabled={!queryKey.trim()} className="btn-primary w-full !text-xs disabled:opacity-40">
              <RefreshCw size={14} /> Query
            </button>
          </div>

          {/* Series Info */}
          <div className="card p-5 space-y-4">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, #8b5cf6 10%, transparent)' }}>
                <Info size={16} style={{ color: '#8b5cf6' }} />
              </div>
              <h3 className="font-display text-sm font-bold" style={{ color: 'var(--text-primary)' }}>Series Info</h3>
            </div>
            <div><label className="text-xs font-semibold mb-1.5 block" style={{ color: 'var(--text-secondary)' }}>Series Key</label>
              <input value={seriesKey} onChange={(e) => setSeriesKey(e.target.value)} className="input-field !text-xs" placeholder="sensor:temperature"
                onKeyDown={(e) => e.key === 'Enter' && handleInfo()} /></div>
            <button onClick={handleInfo} disabled={!seriesKey.trim()} className="btn-secondary w-full !text-xs disabled:opacity-40">
              <Info size={14} /> Get Info
            </button>
          </div>
        </div>

        {/* Right: Chart + Info */}
        <div className="lg:col-span-2 space-y-5">
          {/* Chart */}
          <div className="card overflow-hidden">
            <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <div className="flex items-center gap-2.5">
                <LineChart size={16} style={{ color: 'var(--brand)' }} />
                <h3 className="text-sm font-display font-bold" style={{ color: 'var(--text-primary)' }}>
                  {queryKey ? `${queryKey}` : 'Data Visualization'}
                </h3>
              </div>
              <Badge variant="outline" color="var(--text-tertiary)">{dataPoints.length} points</Badge>
            </div>

            {loading ? <SkeletonChart height={280} /> : !dataPoints.length ? (
              <EmptyState icon={<LineChart size={28} />} title="No data yet" description="Add data points and query a range to visualize your time-series data here." />
            ) : (
              <div className="p-5">
                <svg viewBox={`0 0 ${Math.max(dataPoints.length * 12, 400)} 200`} className="w-full" style={{ height: '280px' }}>
                  {/* Grid */}
                  {[0, 50, 100, 150, 200].map((y) => (
                    <line key={y} x1="0" y1={y} x2="100%" y2={y} stroke="var(--border)" strokeWidth="0.5" strokeDasharray="4 4" />
                  ))}
                  {/* Area */}
                  <path d={`M ${dataPoints.map((d, i) => `${i * 12},${200 - ((d.val - minVal) / range) * 180}`).join(' L ')} L ${(dataPoints.length - 1) * 12},200 L 0,200 Z`}
                    fill="url(#areaGrad)" />
                  {/* Line */}
                  <path d={`M ${dataPoints.map((d, i) => `${i * 12},${200 - ((d.val - minVal) / range) * 180}`).join(' L ')}`}
                    fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinejoin="round" />
                  {/* Dots */}
                  {dataPoints.map((d, i) => (
                    <circle key={i} cx={i * 12} cy={200 - ((d.val - minVal) / range) * 180} r="3" fill="var(--brand)" stroke="var(--bg-primary)" strokeWidth="1.5">
                      <title>{`${fmtTs(d.ts)}: ${d.val}`}</title>
                    </circle>
                  ))}
                  <defs><linearGradient id="areaGrad" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="var(--brand)" stopOpacity="0.2" /><stop offset="100%" stopColor="var(--brand)" stopOpacity="0.01" /></linearGradient></defs>
                </svg>
              </div>
            )}
          </div>

          {/* Info Panel */}
          {infoLoading ? (
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">{Array.from({ length: 4 }).map((_, i) => <SkeletonCard key={i} />)}</div>
          ) : seriesInfo ? (
            <motion.div className="grid grid-cols-2 sm:grid-cols-4 gap-4" variants={container} initial="hidden" animate="show">
              <motion.div variants={item}><StatCard icon={<Database size={16} />} label="Samples" value={String(seriesInfo.total_samples)} color="#f59e0b" /></motion.div>
              <motion.div variants={item}><StatCard icon={<Clock size={16} />} label="First" value={fmtTs(seriesInfo.first_timestamp)} color="#0284c7" /></motion.div>
              <motion.div variants={item}><StatCard icon={<Timer size={16} />} label="Retention" value={seriesInfo.retention_ms ? `${(seriesInfo.retention_ms / 1000).toFixed(0)}s` : '∞'} color="#8b5cf6" /></motion.div>
              <motion.div variants={item}><StatCard icon={<HardDrive size={16} />} label="Memory" value={fmtMem(seriesInfo.memory_bytes)} color="#059669" /></motion.div>
            </motion.div>
          ) : null}
        </div>
      </div>
    </motion.div>
  );
}
