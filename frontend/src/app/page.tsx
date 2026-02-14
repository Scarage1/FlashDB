'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import Link from 'next/link';
import { motion } from 'framer-motion';
import {
  Zap, Database, HardDrive, Clock, Activity, TrendingUp,
  Terminal, Flame, LineChart, Camera, Radio, Gauge,
  ArrowRight, ChevronRight, Search, Shield, GitBranch, Layers, Code,
} from 'lucide-react';
import { getServerInfo, type ServerInfo } from '@/lib/api';
import { FlashLogoMark } from '@/components/ui/Logo';

/* ─── Animated counter ─── */
function AnimatedNumber({ value, format }: { value: number; format?: (n: number) => string }) {
  const [display, setDisplay] = useState(value);
  const prevRef = useRef(value);
  useEffect(() => {
    const prev = prevRef.current; prevRef.current = value;
    if (prev === value) return;
    const diff = value - prev; const steps = 24; let step = 0;
    const id = setInterval(() => { step++; setDisplay(Math.round(prev + diff * (step / steps))); if (step >= steps) clearInterval(id); }, 16);
    return () => clearInterval(id);
  }, [value]);
  return <>{format ? format(display) : display.toLocaleString()}</>;
}

function formatBytes(b: number) { if (b < 1024) return `${b} B`; if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`; return `${(b / (1024 * 1024)).toFixed(1)} MB`; }
function formatUptime(s: number) { if (s < 60) return `${s}s`; if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`; return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`; }

/* ─── Sparkline ─── */
function Sparkline({ data, color, height = 40 }: { data: number[]; color: string; height?: number }) {
  if (data.length < 2) return <div style={{ height }} className="flex items-center justify-center text-xs" />;
  const max = Math.max(...data, 1); const min = Math.min(...data, 0); const range = max - min || 1;
  const w = 200;
  const pts = data.map((v, i) => `${(i / (data.length - 1)) * w},${height - ((v - min) / range) * (height - 4) - 2}`).join(' ');
  return (
    <svg width="100%" height={height} viewBox={`0 0 ${w} ${height}`} preserveAspectRatio="none" className="overflow-visible">
      <defs><linearGradient id={`sp-${color.replace(/[^a-z0-9]/g, '')}`} x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor={color} stopOpacity="0.15" /><stop offset="100%" stopColor={color} stopOpacity="0" /></linearGradient></defs>
      <polygon points={`0,${height} ${pts} ${w},${height}`} fill={`url(#sp-${color.replace(/[^a-z0-9]/g, '')})`} />
      <polyline points={pts} fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/* ─── Feature data ─── */
const features = [
  { icon: <Zap size={20} />, title: 'Sub-ms Latency', desc: 'In-memory architecture with zero-copy reads for blazing fast operations.', color: '#f59e0b' },
  { icon: <Shield size={20} />, title: 'RESP Compatible', desc: 'Drop-in Redis replacement. Use your existing tools and client libraries.', color: '#059669' },
  { icon: <GitBranch size={20} />, title: 'Write-Ahead Log', desc: 'Durable persistence with WAL. Never lose data even on crashes.', color: '#0284c7' },
  { icon: <Activity size={20} />, title: 'Real-time CDC', desc: 'Change Data Capture with SSE streaming for real-time event pipelines.', color: '#ef4444' },
  { icon: <LineChart size={20} />, title: 'Time Series', desc: 'Native time-series data support with aggregation and retention policies.', color: '#8b5cf6' },
  { icon: <Layers size={20} />, title: 'Rich Data Types', desc: 'Strings, hashes, lists, sets, sorted sets, and more.', color: '#ec4899' },
];

const tools = [
  { icon: <Terminal size={18} />, title: 'Console', desc: 'Interactive RESP terminal', href: '/console', color: '#f59e0b' },
  { icon: <Database size={18} />, title: 'Explorer', desc: 'Browse & manage keys', href: '/explorer', color: '#059669' },
  { icon: <Activity size={18} />, title: 'Monitoring', desc: 'Real-time metrics', href: '/monitoring', color: '#0284c7' },
  { icon: <Flame size={18} />, title: 'Hot Keys', desc: 'Access frequency analysis', href: '/hotkeys', color: '#ef4444' },
  { icon: <LineChart size={18} />, title: 'Time Series', desc: 'Temporal data management', href: '/timeseries', color: '#8b5cf6' },
  { icon: <Camera size={18} />, title: 'Snapshots', desc: 'Point-in-time backups', href: '/snapshots', color: '#ec4899' },
  { icon: <Radio size={18} />, title: 'CDC Stream', desc: 'Change data capture', href: '/cdc', color: '#f97316' },
  { icon: <Gauge size={18} />, title: 'Benchmark', desc: 'Performance testing', href: '/benchmark', color: '#10b981' },
];

const codeExample = `import redis

# Connect to FlashDB (Redis-compatible)
client = redis.Redis(host='localhost', port=6379)

# Simple key-value operations
client.set('user:1001', 'Alice')
name = client.get('user:1001')  # b'Alice'

# Rich data structures
client.hset('session:abc', mapping={
    'user': 'alice',
    'role': 'admin',
    'last_seen': '2026-02-13T00:00:00Z'
})

# Sorted sets for leaderboards
client.zadd('leaderboard', {'alice': 9500, 'bob': 8700})
top = client.zrevrange('leaderboard', 0, 9)`;

/* ─── Stagger animation helpers ─── */
const container = { hidden: {}, show: { transition: { staggerChildren: 0.06 } } };
const item = { hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0, transition: { type: 'spring' as const, stiffness: 300, damping: 24 } } };

export default function DashboardPage() {
  const [info, setInfo] = useState<ServerInfo>({ keys: 0, memory: 0, ops: 0, uptime: 0 });
  const [prevInfo, setPrevInfo] = useState<ServerInfo | null>(null);
  const [opsHistory, setOpsHistory] = useState<number[]>([]);
  const [memHistory, setMemHistory] = useState<number[]>([]);
  const [keysHistory, setKeysHistory] = useState<number[]>([]);
  const [connected, setConnected] = useState<boolean | null>(null);

  const fetchInfo = useCallback(async () => {
    const data = await getServerInfo();
    setPrevInfo(info); setInfo(data); setConnected(data.uptime > 0);
    setOpsHistory((h) => [...h.slice(-29), data.ops]);
    setMemHistory((h) => [...h.slice(-29), data.memory]);
    setKeysHistory((h) => [...h.slice(-29), data.keys]);
  }, [info]);

  useEffect(() => { fetchInfo(); const id = setInterval(fetchInfo, 2000); return () => clearInterval(id); }, [fetchInfo]);

  const opsDelta = prevInfo ? info.ops - prevInfo.ops : 0;
  const kpiCards = [
    { label: 'Total Keys', value: info.keys, icon: <Database size={18} />, color: 'var(--brand)', trend: prevInfo ? info.keys - prevInfo.keys : undefined },
    { label: 'Memory Used', value: info.memory, format: formatBytes, icon: <HardDrive size={18} />, color: 'var(--warning)' },
    { label: 'Commands', value: info.ops, icon: <Zap size={18} />, color: 'var(--success)', trend: opsDelta > 0 ? opsDelta : undefined },
    { label: 'Uptime', value: info.uptime, format: formatUptime, icon: <Clock size={18} />, color: 'var(--info)' },
  ];

  return (
    <div>
      {/* ═══ HERO ═══ */}
      <section className="relative overflow-hidden bg-gradient-hero">
        <div className="absolute inset-0 dot-pattern opacity-30" />
        <div className="relative max-w-7xl mx-auto px-4 sm:px-6 pt-16 sm:pt-24 pb-16">
          <motion.div className="text-center max-w-3xl mx-auto"
            initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}>
            {/* Status badge */}
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-semibold mb-6"
              style={{ background: 'var(--brand-muted)', color: 'var(--brand)', border: '1px solid var(--brand-muted)' }}>
              {connected !== null && (
                <span className="relative flex h-1.5 w-1.5">
                  {connected && <span className="absolute inline-flex h-full w-full rounded-full opacity-75" style={{ background: 'var(--success)', animation: 'pulse-dot 2s cubic-bezier(0,0,0.2,1) infinite' }} />}
                  <span className="relative inline-flex rounded-full h-1.5 w-1.5" style={{ background: connected ? 'var(--success)' : 'var(--error)' }} />
                </span>
              )}
              {connected === null ? 'Connecting' : connected ? 'Server Online' : 'Server Offline'} · v2.0
            </div>

            {/* Headline */}
            <h1 className="font-display text-4xl sm:text-5xl lg:text-6xl font-bold tracking-tight leading-[1.1] mb-5">
              Ship features faster with a database that{' '}
              <span className="text-gradient">doesn&apos;t slow you down</span>
            </h1>

            {/* Subtitle */}
            <p className="text-base sm:text-lg leading-relaxed mb-8 max-w-2xl mx-auto" style={{ color: 'var(--text-secondary)' }}>
              FlashDB is a high-performance, Redis-compatible in-memory database built in Go.
              Sub-millisecond latency, durable persistence, and a powerful admin interface.
            </p>

            {/* CTAs */}
            <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
              <Link href="/console" className="btn-primary">
                <Terminal size={16} /> Open Console <ArrowRight size={14} />
              </Link>
              <Link href="/explorer" className="btn-secondary">
                <Search size={16} /> Browse Keys
              </Link>
            </div>
          </motion.div>

          {/* ─── Live KPI Cards ─── */}
          <motion.div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 mt-16 sm:mt-20 max-w-4xl mx-auto"
            variants={container} initial="hidden" animate="show">
            {kpiCards.map((card) => (
              <motion.div key={card.label} variants={item} className="card p-4 sm:p-5 text-center group">
                <div className="flex items-center justify-center mb-3">
                  <span className="w-9 h-9 rounded-lg flex items-center justify-center"
                    style={{ background: `color-mix(in srgb, ${card.color} 10%, transparent)`, color: card.color }}>
                    {card.icon}
                  </span>
                </div>
                <div className="text-2xl sm:text-3xl font-display font-bold tabular-nums" style={{ color: 'var(--text-primary)' }}>
                  <AnimatedNumber value={card.value} format={card.format} />
                </div>
                <div className="flex items-center justify-center gap-1 mt-1">
                  <span className="text-xs font-medium" style={{ color: 'var(--text-tertiary)' }}>{card.label}</span>
                  {card.trend !== undefined && card.trend !== 0 && (
                    <span className="flex items-center text-[10px] font-semibold" style={{ color: card.trend > 0 ? 'var(--success)' : 'var(--error)' }}>
                      <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                        <path d={card.trend > 0 ? "M5 2L8 6H2L5 2Z" : "M5 8L2 4H8L5 8Z"} fill="currentColor" />
                      </svg>
                      {Math.abs(card.trend)}
                    </span>
                  )}
                </div>
              </motion.div>
            ))}
          </motion.div>
        </div>
      </section>

      {/* ═══ TRENDS ═══ */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 py-12">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {[
            { label: 'Commands Trend', data: opsHistory, color: 'var(--success)', icon: <TrendingUp size={15} /> },
            { label: 'Memory Usage', data: memHistory, color: 'var(--warning)', icon: <HardDrive size={15} /> },
            { label: 'Keys Count', data: keysHistory, color: 'var(--brand)', icon: <Database size={15} /> },
          ].map((chart) => (
            <div key={chart.label} className="card p-5">
              <div className="flex items-center gap-2 mb-4">
                <span style={{ color: chart.color }}>{chart.icon}</span>
                <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{chart.label}</span>
              </div>
              <Sparkline data={chart.data} color={chart.color} />
            </div>
          ))}
        </div>
      </section>

      {/* ═══ FEATURES ═══ */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 py-16">
        <div className="text-center mb-12">
          <h2 className="font-display text-3xl sm:text-4xl font-bold tracking-tight mb-3">
            Everything you need, <span className="text-gradient">built-in</span>
          </h2>
          <p className="text-base sm:text-lg max-w-2xl mx-auto" style={{ color: 'var(--text-secondary)' }}>
            A complete database toolkit with enterprise features, zero configuration required.
          </p>
        </div>
        <motion.div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
          variants={container} initial="hidden" whileInView="show" viewport={{ once: true, margin: '-100px' }}>
          {features.map((f) => (
            <motion.div key={f.title} variants={item} className="card-interactive p-6 group">
              <div className="w-11 h-11 rounded-xl flex items-center justify-center mb-4 transition-transform group-hover:scale-110"
                style={{ background: `color-mix(in srgb, ${f.color} 10%, transparent)`, color: f.color }}>
                {f.icon}
              </div>
              <h3 className="font-display text-base font-semibold mb-1.5" style={{ color: 'var(--text-primary)' }}>{f.title}</h3>
              <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>{f.desc}</p>
            </motion.div>
          ))}
        </motion.div>
      </section>

      {/* ═══ CODE EXAMPLE ═══ */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 py-16">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-10 items-center">
          <motion.div initial={{ opacity: 0, x: -20 }} whileInView={{ opacity: 1, x: 0 }} viewport={{ once: true }} transition={{ duration: 0.5 }}>
            <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full text-xs font-semibold mb-4"
              style={{ background: 'var(--accent-muted)', color: 'var(--accent)' }}>
              <Code size={12} /> Developer Experience
            </div>
            <h2 className="font-display text-3xl sm:text-4xl font-bold tracking-tight mb-4">
              Start building in <span className="text-gradient">minutes</span>
            </h2>
            <p className="text-base leading-relaxed mb-6" style={{ color: 'var(--text-secondary)' }}>
              Connect with any Redis client library. FlashDB speaks the RESP protocol natively,
              so you can use your existing tools without changes.
            </p>
            <div className="flex items-center gap-6 mb-6">
              {['Python', 'Go', 'Node.js', 'Java', 'Ruby'].map((lang) => (
                <span key={lang} className="text-xs font-semibold" style={{ color: 'var(--text-tertiary)' }}>{lang}</span>
              ))}
            </div>
            <Link href="/console" className="btn-primary">
              <Terminal size={16} /> Try it now
            </Link>
          </motion.div>
          <motion.div initial={{ opacity: 0, x: 20 }} whileInView={{ opacity: 1, x: 0 }} viewport={{ once: true }} transition={{ duration: 0.5, delay: 0.1 }}
            className="rounded-xl overflow-hidden" style={{ background: '#0c0e14', border: '1px solid var(--border)', boxShadow: 'var(--shadow-xl)' }}>
            <div className="flex items-center gap-2 px-4 py-3" style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
              <div className="flex gap-1.5">
                <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#ef4444' }} />
                <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#f59e0b' }} />
                <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#22c55e' }} />
              </div>
              <span className="text-xs ml-2 font-mono" style={{ color: 'rgba(255,255,255,0.3)' }}>example.py</span>
            </div>
            <pre className="p-5 text-[13px] leading-relaxed overflow-x-auto font-mono" style={{ color: '#e5e7eb' }}>
              <code>{codeExample}</code>
            </pre>
          </motion.div>
        </div>
      </section>

      {/* ═══ TOOLS GRID ═══ */}
      <section className="max-w-7xl mx-auto px-4 sm:px-6 py-16" style={{ borderTop: '1px solid var(--border)' }}>
        <div className="text-center mb-10">
          <h2 className="font-display text-3xl sm:text-4xl font-bold tracking-tight mb-3">
            Powerful <span className="text-gradient">admin tools</span>
          </h2>
          <p className="text-base max-w-xl mx-auto" style={{ color: 'var(--text-secondary)' }}>
            Manage, monitor, and debug your database with built-in tools.
          </p>
        </div>
        <motion.div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3"
          variants={container} initial="hidden" whileInView="show" viewport={{ once: true, margin: '-50px' }}>
          {tools.map((tool) => (
            <motion.div key={tool.href} variants={item}>
              <Link href={tool.href} className="card-interactive p-5 group block h-full">
                <div className="flex items-center justify-between mb-3">
                  <div className="w-10 h-10 rounded-xl flex items-center justify-center transition-transform group-hover:scale-110"
                    style={{ background: `color-mix(in srgb, ${tool.color} 10%, transparent)`, color: tool.color }}>
                    {tool.icon}
                  </div>
                  <ChevronRight size={14} className="opacity-0 group-hover:opacity-100 transition-all group-hover:translate-x-0.5"
                    style={{ color: 'var(--text-tertiary)' }} />
                </div>
                <h3 className="text-sm font-semibold mb-0.5 font-display" style={{ color: 'var(--text-primary)' }}>{tool.title}</h3>
                <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{tool.desc}</p>
              </Link>
            </motion.div>
          ))}
        </motion.div>
      </section>

      {/* ═══ FOOTER ═══ */}
      <footer className="max-w-7xl mx-auto px-4 sm:px-6 py-8" style={{ borderTop: '1px solid var(--border)' }}>
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2.5">
            <FlashLogoMark size={18} />
            <span className="font-display text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>FlashDB</span>
            <span className="text-xs font-mono px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-tertiary)' }}>v2.0.0</span>
          </div>
          <div className="flex items-center gap-5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
            <a href="https://github.com/Scarage1/FlashDB" target="_blank" rel="noopener noreferrer" className="hover:underline" style={{ color: 'var(--text-secondary)' }}>GitHub</a>
            <Link href="/settings" className="hover:underline" style={{ color: 'var(--text-secondary)' }}>Settings</Link>
            <span>Built with Go + Next.js</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
