'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { motion } from 'framer-motion';
import { executeCommand, type CommandResponse } from '@/lib/api';
import PageHeader from '@/components/ui/PageHeader';
import EmptyState from '@/components/ui/EmptyState';
import { Terminal, Play, Trash2, Download, Copy, Check, ChevronRight } from 'lucide-react';

const COMMANDS = [
  'SET','GET','DEL','MGET','MSET','APPEND','INCR','DECR','INCRBY','DECRBY','STRLEN',
  'EXPIRE','TTL','PTTL','PERSIST','TYPE','RENAME','EXISTS','KEYS','SCAN','RANDOMKEY',
  'DBSIZE','FLUSHDB','PING','ECHO','INFO','SELECT','CONFIG',
  'HSET','HGET','HDEL','HGETALL','HKEYS','HVALS','HLEN','HEXISTS','HMSET','HMGET','HINCRBY',
  'LPUSH','RPUSH','LPOP','RPOP','LRANGE','LLEN','LINDEX','LSET','LREM','LTRIM',
  'SADD','SREM','SMEMBERS','SISMEMBER','SCARD','SUNION','SINTER','SDIFF','SRANDMEMBER','SPOP',
  'ZADD','ZREM','ZSCORE','ZRANK','ZRANGE','ZREVRANGE','ZRANGEBYSCORE','ZCARD','ZCOUNT','ZINCRBY',
  'AUTH','ACL','SLOWLOG','MULTI','EXEC','DISCARD',
];

function highlightCommand(text: string): React.ReactNode {
  const parts = text.split(/\s+/);
  if (!parts.length) return text;
  const cmd = parts[0].toUpperCase();
  const isKnown = COMMANDS.includes(cmd);
  return (
    <span>
      <span style={{ color: isKnown ? 'var(--brand)' : 'var(--error)', fontWeight: 600 }}>{parts[0]}</span>
      {parts.slice(1).map((p, i) => (
        <span key={i}>{' '}<span style={{ color: p.startsWith('"') || p.startsWith("'") ? '#34d399' : 'var(--console-text)' }}>{p}</span></span>
      ))}
    </span>
  );
}

interface HistoryEntry { id: number; command: string; result: string; isError: boolean; timestamp: number; }

function formatResult(result: unknown): string {
  if (result === null || result === undefined) return '(nil)';
  if (typeof result === 'string') return result;
  if (Array.isArray(result)) { if (!result.length) return '(empty array)'; return result.map((v, i) => `${i + 1}) ${JSON.stringify(v)}`).join('\n'); }
  if (typeof result === 'object') return JSON.stringify(result, null, 2);
  return String(result);
}

export default function ConsolePage() {
  const [input, setInput] = useState('');
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [copiedId, setCopiedId] = useState<number | null>(null);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [selSugg, setSelSugg] = useState(0);
  const outputRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const idRef = useRef(0);

  useEffect(() => { try { const s = localStorage.getItem('flashdb-console-history'); if (s) setHistory(JSON.parse(s)); } catch {} }, []);
  useEffect(() => { try { localStorage.setItem('flashdb-console-history', JSON.stringify(history.slice(-200))); } catch {} }, [history]);
  useEffect(() => { if (outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight; }, [history]);
  useEffect(() => {
    const word = input.split(/\s+/)[0]?.toUpperCase() || '';
    if (word.length > 0) { setSuggestions(COMMANDS.filter((c) => c.startsWith(word) && c !== word).slice(0, 6)); setSelSugg(0); }
    else setSuggestions([]);
  }, [input]);

  const runCommand = useCallback(async () => {
    const cmd = input.trim();
    if (!cmd) return;
    const entry: HistoryEntry = { id: ++idRef.current, command: cmd, result: '', isError: false, timestamp: Date.now() };
    setInput(''); setHistoryIndex(-1); setSuggestions([]);
    const res: CommandResponse = await executeCommand(cmd);
    entry.result = res.error || formatResult(res.result);
    entry.isError = !!res.error;
    setHistory((h) => [...h, entry]);
  }, [input]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (suggestions.length > 0) {
      if (e.key === 'Tab') { e.preventDefault(); const p = input.split(/\s+/); p[0] = suggestions[selSugg]; setInput(p.join(' ') + ' '); setSuggestions([]); return; }
      if (e.key === 'ArrowDown') { e.preventDefault(); setSelSugg((s) => Math.min(s + 1, suggestions.length - 1)); return; }
      if (e.key === 'ArrowUp') { e.preventDefault(); setSelSugg((s) => Math.max(s - 1, 0)); return; }
    }
    if (e.key === 'Enter') { e.preventDefault(); runCommand(); }
    else if (e.key === 'ArrowUp' && !suggestions.length) { e.preventDefault(); const cmds = history.map(h => h.command); const ni = historyIndex + 1; if (ni < cmds.length) { setHistoryIndex(ni); setInput(cmds[cmds.length - 1 - ni]); } }
    else if (e.key === 'ArrowDown' && !suggestions.length) { e.preventDefault(); const cmds = history.map(h => h.command); const ni = historyIndex - 1; if (ni >= 0) { setHistoryIndex(ni); setInput(cmds[cmds.length - 1 - ni]); } else { setHistoryIndex(-1); setInput(''); } }
  };

  return (
    <motion.div className="max-w-5xl mx-auto px-4 sm:px-6 py-8"
      initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }}>
      <PageHeader title="Console" description={`Execute RESP commands · ${history.length} in history`}
        actions={<>
          <button onClick={() => { const t = history.map(e => `> ${e.command}\n${e.result}`).join('\n\n'); const b = new Blob([t], { type: 'text/plain' }); const u = URL.createObjectURL(b); const a = document.createElement('a'); a.href = u; a.download = `flashdb-console-${new Date().toISOString().slice(0,10)}.txt`; a.click(); URL.revokeObjectURL(u); }}
            className="btn-secondary !px-3 !py-1.5 !text-xs"><Download size={14} /> Export</button>
          <button onClick={() => { setHistory([]); localStorage.removeItem('flashdb-console-history'); }}
            className="btn-secondary !px-3 !py-1.5 !text-xs" style={{ color: 'var(--error)' }}><Trash2 size={14} /> Clear</button>
        </>}
      />

      {/* Console */}
      <div className="rounded-xl overflow-hidden" style={{ background: 'var(--console-bg)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }}>
        {/* Top bar */}
        <div className="flex items-center justify-between px-5 py-3" style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
          <div className="flex items-center gap-3">
            <div className="flex gap-1.5">
              <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#ef4444' }} />
              <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#f59e0b' }} />
              <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#22c55e' }} />
            </div>
            <span className="text-xs font-mono" style={{ color: 'rgba(255,255,255,0.25)' }}>FlashDB Console — localhost:6379</span>
          </div>
          <Terminal size={14} style={{ color: 'rgba(255,255,255,0.15)' }} />
        </div>

        {/* Output */}
        <div ref={outputRef} className="console-output px-5 py-5 font-mono text-sm space-y-3 overflow-y-auto"
          style={{ height: 'calc(100vh - 360px)', minHeight: '300px' }}>
          {!history.length && (
            <EmptyState icon={<Terminal size={28} />}
              title="Ready for commands"
              description="Type a RESP command to get started. Try PING, SET key value, or INFO." />
          )}
          {history.map((entry) => (
            <div key={entry.id} className="group">
              <div className="flex items-center gap-2">
                <ChevronRight size={11} style={{ color: 'var(--brand)' }} />
                <span>{highlightCommand(entry.command)}</span>
                <button onClick={() => { navigator.clipboard.writeText(entry.result); setCopiedId(entry.id); setTimeout(() => setCopiedId(null), 1500); }}
                  className="opacity-0 group-hover:opacity-100 transition-opacity ml-auto" style={{ color: 'rgba(255,255,255,0.2)' }}>
                  {copiedId === entry.id ? <Check size={11} /> : <Copy size={11} />}
                </button>
              </div>
              <pre className="mt-1 ml-5 whitespace-pre-wrap text-xs leading-relaxed"
                style={{ color: entry.isError ? 'var(--error)' : 'rgba(255,255,255,0.45)' }}>{entry.result}</pre>
            </div>
          ))}
        </div>

        {/* Input */}
        <div className="relative" style={{ borderTop: '1px solid rgba(255,255,255,0.06)' }}>
          {suggestions.length > 0 && (
            <div className="absolute bottom-full left-0 right-0 p-1.5" style={{ background: '#1e293b', borderTop: '1px solid rgba(255,255,255,0.06)' }}>
              {suggestions.map((s, i) => (
                <button key={s} className="block w-full text-left px-3 py-1.5 text-xs font-mono rounded-md transition-colors"
                  style={{ background: i === selSugg ? 'var(--brand-muted)' : 'transparent', color: i === selSugg ? 'var(--brand)' : '#94a3b8' }}
                  onClick={() => { const p = input.split(/\s+/); p[0] = s; setInput(p.join(' ') + ' '); setSuggestions([]); inputRef.current?.focus(); }}
                  onMouseEnter={() => setSelSugg(i)}>{s}</button>
              ))}
            </div>
          )}
          <div className="flex items-center px-5 py-3.5 gap-3">
            <span className="font-mono text-sm font-bold" style={{ color: 'var(--brand)' }}>&gt;</span>
            <input ref={inputRef} value={input} onChange={(e) => setInput(e.target.value)} onKeyDown={handleKeyDown}
              placeholder="Enter command…" className="flex-1 bg-transparent font-mono text-sm outline-none"
              style={{ color: 'var(--console-text)' }} spellCheck={false} autoComplete="off" />
            <button onClick={runCommand} disabled={!input.trim()} className="btn-primary !px-3 !py-1.5 !text-xs disabled:opacity-30">
              <Play size={12} /> Run
            </button>
          </div>
        </div>
      </div>
    </motion.div>
  );
}
