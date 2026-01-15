'use client';

import { useState, useEffect } from 'react';
import { Zap, Terminal, Database, BookOpen } from 'lucide-react';

export default function Navbar() {
  const [scrolled, setScrolled] = useState(false);
  const [connected, setConnected] = useState(true);

  useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 20);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const scrollTo = (id: string) => {
    const element = document.getElementById(id);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  return (
    <nav
      className={`sticky top-0 z-50 glass border-b transition-all duration-300 ${
        scrolled ? 'shadow-sm border-black/10' : 'border-transparent'
      }`}
    >
      <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
        {/* Brand */}
        <a href="#" className="flex items-center gap-3 group">
          <div className="w-9 h-9 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-xl flex items-center justify-center shadow-lg shadow-blue-500/25 group-hover:scale-110 transition-transform">
            <Zap className="w-5 h-5 text-white" />
          </div>
          <span className="font-semibold text-xl tracking-tight">FlashDB</span>
        </a>

        {/* Navigation */}
        <ul className="hidden md:flex items-center gap-1">
          {[
            { id: 'console', label: 'Console', icon: Terminal },
            { id: 'keys', label: 'Keys', icon: Database },
            { id: 'features', label: 'Features', icon: Zap },
            { id: 'docs', label: 'Docs', icon: BookOpen },
          ].map(({ id, label, icon: Icon }) => (
            <li key={id}>
              <button
                onClick={() => scrollTo(id)}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 hover:bg-black/5 rounded-full transition-all"
              >
                <Icon className="w-4 h-4" />
                {label}
              </button>
            </li>
          ))}
        </ul>

        {/* Status */}
        <div className="flex items-center gap-2 px-4 py-2 bg-white border border-gray-200 rounded-full shadow-sm">
          <span
            className={`w-2 h-2 rounded-full ${
              connected ? 'bg-green-500 animate-pulse-soft' : 'bg-red-500'
            }`}
          />
          <span className="text-sm text-gray-600">
            {connected ? 'Connected' : 'Disconnected'}
          </span>
        </div>
      </div>
    </nav>
  );
}
