'use client';

import { useState, useEffect } from 'react';
import { Database, HardDrive, Activity, Clock } from 'lucide-react';
import { getServerInfo } from '@/lib/api';

interface StatCardProps {
  icon: React.ReactNode;
  value: string;
  label: string;
  delay: number;
}

function StatCard({ icon, value, label, delay }: StatCardProps) {
  return (
    <div
      className="group bg-white border border-gray-200 rounded-3xl p-7 shadow-md hover:shadow-xl hover:-translate-y-1 transition-all duration-300 relative overflow-hidden animate-slide-up"
      style={{ animationDelay: `${delay}s` }}
    >
      {/* Top accent line */}
      <div className="absolute top-0 left-0 right-0 h-1 bg-gradient-to-r from-blue-500 to-cyan-400 transform -translate-x-full group-hover:translate-x-0 transition-transform duration-500" />
      
      {/* Icon */}
      <div className="w-12 h-12 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-2xl flex items-center justify-center shadow-lg shadow-blue-500/20 mb-4">
        {icon}
      </div>
      
      {/* Value */}
      <div className="text-4xl font-bold bg-gradient-to-r from-purple-600 to-blue-500 bg-clip-text text-transparent mb-1">
        {value}
      </div>
      
      {/* Label */}
      <div className="text-sm text-gray-500 font-medium">{label}</div>
    </div>
  );
}

export default function StatsGrid() {
  const [stats, setStats] = useState({
    keys: 0,
    memory: '0 MB',
    ops: 0,
    uptime: '0s',
  });

  function formatMemory(bytes?: number) {
    if (!bytes) return '0 MB';
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  }

  function formatUptime(seconds?: number) {
    if (!seconds) return '0s';
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
    return `${Math.floor(seconds / 86400)}d`;
  }

  useEffect(() => {
    const fetchStats = async () => {
      const info = await getServerInfo();
      setStats({
        keys: info.keys || 0,
        memory: formatMemory(info.memory),
        ops: info.ops || 0,
        uptime: formatUptime(info.uptime),
      });
    };

    fetchStats();
    const interval = setInterval(fetchStats, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 max-w-6xl mx-auto px-6 pb-16">
      <StatCard
        icon={<Database className="w-6 h-6 text-white" />}
        value={stats.keys.toString()}
        label="Total Keys"
        delay={0}
      />
      <StatCard
        icon={<HardDrive className="w-6 h-6 text-white" />}
        value={stats.memory}
        label="Memory Usage"
        delay={0.1}
      />
      <StatCard
        icon={<Activity className="w-6 h-6 text-white" />}
        value={stats.ops.toString()}
        label="Operations/sec"
        delay={0.2}
      />
      <StatCard
        icon={<Clock className="w-6 h-6 text-white" />}
        value={stats.uptime}
        label="Uptime"
        delay={0.3}
      />
    </div>
  );
}
