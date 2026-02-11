'use client';

import { useState, useRef, useEffect } from 'react';
import { Trash2 } from 'lucide-react';
import { executeCommand } from '@/lib/api';

interface ConsoleLine {
  type: 'command' | 'result' | 'error' | 'info';
  text: string;
}

export default function Console() {
  const [input, setInput] = useState('');
  const [lines, setLines] = useState<ConsoleLine[]>([
    { type: 'info', text: 'Welcome to FlashDB Console! Type your commands below.' },
    { type: 'info', text: 'Try: INFO, PING, SET mykey "value", GET mykey' },
  ]);
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const outputRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight;
    }
  }, [lines]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim()) return;

    const command = input.trim();
    setLines((prev) => [...prev, { type: 'command', text: `flashdb> ${command}` }]);
    setHistory((prev) => [...prev, command]);
    setHistoryIndex(-1);
    setInput('');

    const response = await executeCommand(command);
    
    if (response.error) {
      setLines((prev) => [...prev, { type: 'error', text: `Error: ${response.error}` }]);
    } else {
      const result = formatResult(response.result);
      setLines((prev) => [...prev, { type: 'result', text: result }]);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (historyIndex < history.length - 1) {
        const newIndex = historyIndex + 1;
        setHistoryIndex(newIndex);
        setInput(history[history.length - 1 - newIndex]);
      }
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (historyIndex > 0) {
        const newIndex = historyIndex - 1;
        setHistoryIndex(newIndex);
        setInput(history[history.length - 1 - newIndex]);
      } else {
        setHistoryIndex(-1);
        setInput('');
      }
    }
  };

  const formatResult = (result: unknown): string => {
    if (result === null || result === undefined) return '(nil)';
    if (typeof result === 'object') return JSON.stringify(result, null, 2);
    return String(result);
  };

  const clearConsole = () => {
    setLines([{ type: 'info', text: 'Console cleared.' }]);
  };

  return (
    <section id="console" className="mb-16">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-3xl font-bold tracking-tight">Interactive Console</h2>
        <button
          onClick={clearConsole}
          className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-600 bg-white border border-gray-200 rounded-full hover:bg-gray-50 transition-colors"
        >
          <Trash2 className="w-4 h-4" />
          Clear
        </button>
      </div>

      {/* Console Card */}
      <div className="bg-[#1d1d1f] rounded-3xl overflow-hidden shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-white/10">
          <div className="flex items-center gap-3">
            <span className="text-white/60 text-sm font-medium">⚡ FlashDB Terminal</span>
          </div>
          <div className="flex gap-2">
            <div className="w-3 h-3 rounded-full bg-[#ff5f57] hover:scale-110 transition-transform cursor-pointer" />
            <div className="w-3 h-3 rounded-full bg-[#ffbd2e] hover:scale-110 transition-transform cursor-pointer" />
            <div className="w-3 h-3 rounded-full bg-[#28c840] hover:scale-110 transition-transform cursor-pointer" />
          </div>
        </div>

        {/* Input */}
        <form onSubmit={handleSubmit} className="flex items-center gap-3 px-6 py-5 border-b border-white/10">
          <span className="text-cyan-400 font-mono font-semibold select-none">flashdb&gt;</span>
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a command (e.g., SET mykey 'hello world')"
            className="flex-1 bg-transparent text-white font-mono text-sm outline-none placeholder:text-white/30"
            autoComplete="off"
            spellCheck={false}
          />
        </form>

        {/* Output */}
        <div
          ref={outputRef}
          className="console-output px-6 py-5 h-96 overflow-y-auto font-mono text-sm space-y-3"
        >
          {lines.map((line, i) => (
            <div
              key={i}
              className={`animate-fade-in ${
                line.type === 'command'
                  ? 'text-cyan-400'
                  : line.type === 'result'
                  ? 'text-green-400 pl-5'
                  : line.type === 'error'
                  ? 'text-red-400 pl-5'
                  : 'text-white/60'
              }`}
            >
              {line.text}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
