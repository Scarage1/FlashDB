'use client';

import { useState, useEffect, useCallback } from 'react';
import { motion } from 'framer-motion';
import { Settings as SettingsIcon, Server, RefreshCw, Database, Shield, ExternalLink, Github, FileText, BookOpen, MessageSquare } from 'lucide-react';
import { executeCommand } from '@/lib/api';
import PageHeader from '@/components/ui/PageHeader';
import Badge from '@/components/ui/Badge';
import EmptyState from '@/components/ui/EmptyState';
import { SkeletonTable } from '@/components/ui/Skeleton';

function fmtMem(b: string | number) { const n = Number(b); if (n >= 1e9) return `${(n / 1e9).toFixed(1)} GB`; if (n >= 1e6) return `${(n / 1e6).toFixed(1)} MB`; if (n >= 1e3) return `${(n / 1e3).toFixed(1)} KB`; return `${n} B`; }

const container = { hidden: {}, show: { transition: { staggerChildren: 0.04 } } };
const item = { hidden: { opacity: 0, y: 8 }, show: { opacity: 1, y: 0 } };

const RESOURCES = [
  { label: 'GitHub Repository', desc: 'Source code & issues', icon: <Github size={16} />, url: 'https://github.com/shivamkumar4344/FlashDB', color: '#f59e0b' },
  { label: 'Documentation', desc: 'Guides & API reference', icon: <BookOpen size={16} />, url: 'https://github.com/shivamkumar4344/FlashDB/tree/main/docs', color: '#0284c7' },
  { label: 'RESP Protocol', desc: 'Protocol specification', icon: <FileText size={16} />, url: 'https://github.com/shivamkumar4344/FlashDB/blob/main/docs/PROTOCOL.md', color: '#8b5cf6' },
  { label: 'Contributing', desc: 'How to contribute', icon: <MessageSquare size={16} />, url: 'https://github.com/shivamkumar4344/FlashDB/blob/main/CONTRIBUTING.md', color: '#22c55e' },
];

export default function SettingsPage() {
  const [info, setInfo] = useState<Record<string, string>>({});
  const [aclUsers, setAclUsers] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchAll = useCallback(async () => {
    setLoading(true);
    const infoRes = await executeCommand('INFO');
    const map: Record<string, string> = {};
    const raw = typeof infoRes.result === 'string' ? infoRes.result : '';
    raw.split('\n').forEach((line: string) => {
      if (!line || line.startsWith('#')) return;
      const idx = line.indexOf(':');
      if (idx > 0) map[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
    });
    setInfo(map);

    const aclRes = await executeCommand('ACL LIST');
    if (Array.isArray(aclRes.result)) setAclUsers(aclRes.result.map(String));
    else if (typeof aclRes.result === 'string' && aclRes.result) setAclUsers(aclRes.result.split('\n').filter(Boolean));
    else setAclUsers([]);

    setLoading(false);
  }, []);

  useEffect(() => { fetchAll(); }, [fetchAll]);

  const serverProps = [
    { label: 'Version', value: info.flashdb_version || '—' },
    { label: 'Uptime (seconds)', value: info.uptime_in_seconds || '—' },
    { label: 'DB Keys', value: info.keys || '—' },
    { label: 'Total Reads', value: info.total_reads ? Number(info.total_reads).toLocaleString() : '—' },
    { label: 'Total Writes', value: info.total_writes ? Number(info.total_writes).toLocaleString() : '—' },
    { label: 'Used Memory', value: info.used_memory ? fmtMem(info.used_memory) : '—' },
  ];

  return (
    <motion.div className="max-w-5xl mx-auto px-4 sm:px-6 py-8" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.35 }}>
      <PageHeader title="Settings" description="Server configuration, security, and resources"
        actions={<button onClick={fetchAll} className="btn-secondary !px-3 !py-1.5 !text-xs"><RefreshCw size={14} className={loading ? 'animate-spin' : ''} /> Refresh</button>} />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Server Info */}
        <div className="lg:col-span-2">
          <div className="card overflow-hidden">
            <div className="flex items-center gap-2.5 px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, var(--brand) 10%, transparent)' }}>
                <Server size={16} style={{ color: 'var(--brand)' }} />
              </div>
              <div>
                <h3 className="font-display text-sm font-bold" style={{ color: 'var(--text-primary)' }}>Server Information</h3>
                <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>INFO command output</p>
              </div>
            </div>

            {loading ? <SkeletonTable rows={8} /> : (
              <motion.div className="divide-y" style={{ borderColor: 'var(--border)' }} variants={container} initial="hidden" animate="show">
                {serverProps.map((prop) => (
                  <motion.div key={prop.label} variants={item} className="flex items-center justify-between px-5 py-3 transition-colors"
                    onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-secondary)'}
                    onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
                    <span className="text-xs font-semibold" style={{ color: 'var(--text-secondary)' }}>{prop.label}</span>
                    <span className="text-xs font-mono" style={{ color: 'var(--text-primary)' }}>{prop.value}</span>
                  </motion.div>
                ))}
              </motion.div>
            )}
          </div>
        </div>

        {/* Right column */}
        <div className="space-y-6">
          {/* ACL */}
          <div className="card overflow-hidden">
            <div className="flex items-center gap-2.5 px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'color-mix(in srgb, #ef4444 10%, transparent)' }}>
                <Shield size={16} style={{ color: '#ef4444' }} />
              </div>
              <div>
                <h3 className="font-display text-sm font-bold" style={{ color: 'var(--text-primary)' }}>Access Control</h3>
                <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>ACL users</p>
              </div>
            </div>

            {loading ? <SkeletonTable rows={3} /> : !aclUsers.length ? (
              <EmptyState icon={<Shield size={20} />} title="No ACL" description="No ACL rules configured." />
            ) : (
              <div className="p-3 space-y-1">
                {aclUsers.map((u, i) => (
                  <div key={i} className="px-4 py-2.5 rounded-lg text-xs font-mono" style={{ background: 'var(--bg-secondary)', color: 'var(--text-primary)' }}>{u}</div>
                ))}
              </div>
            )}
          </div>

          {/* Resources */}
          <div className="card overflow-hidden">
            <div className="flex items-center gap-2.5 px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <Database size={16} style={{ color: 'var(--brand)' }} />
              <h3 className="font-display text-sm font-bold" style={{ color: 'var(--text-primary)' }}>Resources</h3>
            </div>
            <div className="p-3 space-y-1">
              {RESOURCES.map((r) => (
                <a key={r.label} href={r.url} target="_blank" rel="noopener noreferrer"
                  className="flex items-center gap-3 px-4 py-3 rounded-lg transition-colors group"
                  onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-secondary)'}
                  onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}>
                  <div className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0" style={{ background: `color-mix(in srgb, ${r.color} 10%, transparent)` }}>
                    <span style={{ color: r.color }}>{r.icon}</span>
                  </div>
                  <div className="flex-1">
                    <p className="text-xs font-semibold" style={{ color: 'var(--text-primary)' }}>{r.label}</p>
                    <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{r.desc}</p>
                  </div>
                  <ExternalLink size={12} style={{ color: 'var(--text-tertiary)' }} className="opacity-0 group-hover:opacity-100 transition-opacity" />
                </a>
              ))}
            </div>
          </div>
        </div>
      </div>
    </motion.div>
  );
}
