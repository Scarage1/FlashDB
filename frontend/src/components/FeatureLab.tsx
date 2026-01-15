'use client';

import { useState } from 'react';
import { Type, Clock, Hash, BarChart3 } from 'lucide-react';
import { executeCommand } from '@/lib/api';
import { useToast } from '@/context/ToastContext';

type TabType = 'strings' | 'ttl' | 'counters' | 'sortedsets';

interface TabButtonProps {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
}

function TabButton({ active, onClick, icon, label }: TabButtonProps) {
  return (
    <button
      onClick={onClick}
      className={`flex-1 flex items-center justify-center gap-2 py-3 text-sm font-semibold rounded-xl transition-all ${
        active
          ? 'bg-white text-gray-900 shadow-sm'
          : 'text-gray-500 hover:text-gray-700'
      }`}
    >
      {icon}
      {label}
    </button>
  );
}

export default function FeatureLab() {
  const [activeTab, setActiveTab] = useState<TabType>('strings');
  const { showToast } = useToast();

  // Form states
  const [stringKey, setStringKey] = useState('');
  const [stringValue, setStringValue] = useState('');
  const [ttlKey, setTtlKey] = useState('');
  const [ttlSeconds, setTtlSeconds] = useState('');
  const [counterKey, setCounterKey] = useState('');
  const [counterDelta, setCounterDelta] = useState('1');
  const [zsetKey, setZsetKey] = useState('');
  const [zsetMember, setZsetMember] = useState('');
  const [zsetScore, setZsetScore] = useState('');

  const runCommand = async (cmd: string, successMsg: string) => {
    const result = await executeCommand(cmd);
    if (result.error) {
      showToast(`Error: ${result.error}`, 'error');
    } else {
      showToast(successMsg, 'success');
    }
  };

  // String operations
  const handleSet = () => {
    if (!stringKey || !stringValue) return showToast('Fill in all fields', 'error');
    runCommand(`SET ${stringKey} "${stringValue}"`, 'Key set successfully');
  };

  const handleGet = () => {
    if (!stringKey) return showToast('Enter a key name', 'error');
    runCommand(`GET ${stringKey}`, 'Value retrieved');
  };

  const handleDel = () => {
    if (!stringKey) return showToast('Enter a key name', 'error');
    runCommand(`DEL ${stringKey}`, 'Key deleted');
  };

  // TTL operations
  const handleExpire = () => {
    if (!ttlKey || !ttlSeconds) return showToast('Fill in all fields', 'error');
    runCommand(`EXPIRE ${ttlKey} ${ttlSeconds}`, 'TTL set successfully');
  };

  const handleTTL = () => {
    if (!ttlKey) return showToast('Enter a key name', 'error');
    runCommand(`TTL ${ttlKey}`, 'TTL retrieved');
  };

  // Counter operations
  const handleIncr = () => {
    if (!counterKey) return showToast('Enter a key name', 'error');
    runCommand(`INCR ${counterKey}`, 'Counter incremented');
  };

  const handleIncrBy = () => {
    if (!counterKey) return showToast('Enter a key name', 'error');
    runCommand(`INCRBY ${counterKey} ${counterDelta || 1}`, 'Counter incremented');
  };

  const handleDecr = () => {
    if (!counterKey) return showToast('Enter a key name', 'error');
    runCommand(`DECR ${counterKey}`, 'Counter decremented');
  };

  // Sorted set operations
  const handleZAdd = () => {
    if (!zsetKey || !zsetMember || !zsetScore) return showToast('Fill in all fields', 'error');
    runCommand(`ZADD ${zsetKey} ${zsetScore} ${zsetMember}`, 'Member added to sorted set');
  };

  const handleZScore = () => {
    if (!zsetKey || !zsetMember) return showToast('Enter key and member', 'error');
    runCommand(`ZSCORE ${zsetKey} ${zsetMember}`, 'Score retrieved');
  };

  const handleZRange = () => {
    if (!zsetKey) return showToast('Enter a key name', 'error');
    runCommand(`ZRANGE ${zsetKey} 0 -1 WITHSCORES`, 'Range retrieved');
  };

  return (
    <section id="features" className="mb-16">
      <h2 className="text-3xl font-bold tracking-tight mb-6">Feature Lab</h2>

      <div className="bg-white border border-gray-200 rounded-3xl p-8 shadow-md">
        {/* Tabs */}
        <div className="flex gap-1 p-1 bg-gray-100 rounded-2xl mb-8">
          <TabButton
            active={activeTab === 'strings'}
            onClick={() => setActiveTab('strings')}
            icon={<Type className="w-4 h-4" />}
            label="Strings"
          />
          <TabButton
            active={activeTab === 'ttl'}
            onClick={() => setActiveTab('ttl')}
            icon={<Clock className="w-4 h-4" />}
            label="TTL & Expiry"
          />
          <TabButton
            active={activeTab === 'counters'}
            onClick={() => setActiveTab('counters')}
            icon={<Hash className="w-4 h-4" />}
            label="Counters"
          />
          <TabButton
            active={activeTab === 'sortedsets'}
            onClick={() => setActiveTab('sortedsets')}
            icon={<BarChart3 className="w-4 h-4" />}
            label="Sorted Sets"
          />
        </div>

        {/* Strings Tab */}
        {activeTab === 'strings' && (
          <div className="animate-fade-in">
            <h3 className="text-xl font-semibold mb-5">String Operations</h3>
            <div className="grid md:grid-cols-2 gap-4 mb-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Key</label>
                <input
                  type="text"
                  value={stringKey}
                  onChange={(e) => setStringKey(e.target.value)}
                  placeholder="mykey"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Value</label>
                <input
                  type="text"
                  value={stringValue}
                  onChange={(e) => setStringValue(e.target.value)}
                  placeholder="hello world"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
            </div>
            <div className="flex gap-3">
              <button onClick={handleSet} className="px-6 py-3 text-sm font-semibold text-white bg-gradient-to-r from-blue-600 to-cyan-500 rounded-xl shadow-lg shadow-blue-500/25 hover:shadow-xl transition-all">
                SET
              </button>
              <button onClick={handleGet} className="px-6 py-3 text-sm font-semibold text-gray-700 bg-gray-100 rounded-xl hover:bg-gray-200 transition-colors">
                GET
              </button>
              <button onClick={handleDel} className="px-6 py-3 text-sm font-semibold text-red-600 bg-red-50 rounded-xl hover:bg-red-100 transition-colors">
                DEL
              </button>
            </div>
          </div>
        )}

        {/* TTL Tab */}
        {activeTab === 'ttl' && (
          <div className="animate-fade-in">
            <h3 className="text-xl font-semibold mb-5">TTL & Expiration</h3>
            <div className="grid md:grid-cols-2 gap-4 mb-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Key</label>
                <input
                  type="text"
                  value={ttlKey}
                  onChange={(e) => setTtlKey(e.target.value)}
                  placeholder="mykey"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Seconds</label>
                <input
                  type="number"
                  value={ttlSeconds}
                  onChange={(e) => setTtlSeconds(e.target.value)}
                  placeholder="60"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
            </div>
            <div className="flex gap-3">
              <button onClick={handleExpire} className="px-6 py-3 text-sm font-semibold text-white bg-gradient-to-r from-blue-600 to-cyan-500 rounded-xl shadow-lg shadow-blue-500/25 hover:shadow-xl transition-all">
                EXPIRE
              </button>
              <button onClick={handleTTL} className="px-6 py-3 text-sm font-semibold text-gray-700 bg-gray-100 rounded-xl hover:bg-gray-200 transition-colors">
                TTL
              </button>
            </div>
          </div>
        )}

        {/* Counters Tab */}
        {activeTab === 'counters' && (
          <div className="animate-fade-in">
            <h3 className="text-xl font-semibold mb-5">Counter Operations</h3>
            <div className="grid md:grid-cols-2 gap-4 mb-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Counter Key</label>
                <input
                  type="text"
                  value={counterKey}
                  onChange={(e) => setCounterKey(e.target.value)}
                  placeholder="visits"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Delta (for INCRBY)</label>
                <input
                  type="number"
                  value={counterDelta}
                  onChange={(e) => setCounterDelta(e.target.value)}
                  placeholder="1"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
            </div>
            <div className="flex gap-3">
              <button onClick={handleIncr} className="px-6 py-3 text-sm font-semibold text-white bg-gradient-to-r from-blue-600 to-cyan-500 rounded-xl shadow-lg shadow-blue-500/25 hover:shadow-xl transition-all">
                INCR
              </button>
              <button onClick={handleIncrBy} className="px-6 py-3 text-sm font-semibold text-gray-700 bg-gray-100 rounded-xl hover:bg-gray-200 transition-colors">
                INCRBY
              </button>
              <button onClick={handleDecr} className="px-6 py-3 text-sm font-semibold text-orange-600 bg-orange-50 rounded-xl hover:bg-orange-100 transition-colors">
                DECR
              </button>
            </div>
          </div>
        )}

        {/* Sorted Sets Tab */}
        {activeTab === 'sortedsets' && (
          <div className="animate-fade-in">
            <h3 className="text-xl font-semibold mb-5">Sorted Set Operations</h3>
            <div className="grid md:grid-cols-3 gap-4 mb-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Key</label>
                <input
                  type="text"
                  value={zsetKey}
                  onChange={(e) => setZsetKey(e.target.value)}
                  placeholder="leaderboard"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Member</label>
                <input
                  type="text"
                  value={zsetMember}
                  onChange={(e) => setZsetMember(e.target.value)}
                  placeholder="player1"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Score</label>
                <input
                  type="number"
                  value={zsetScore}
                  onChange={(e) => setZsetScore(e.target.value)}
                  placeholder="100"
                  className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
                />
              </div>
            </div>
            <div className="flex gap-3">
              <button onClick={handleZAdd} className="px-6 py-3 text-sm font-semibold text-white bg-gradient-to-r from-blue-600 to-cyan-500 rounded-xl shadow-lg shadow-blue-500/25 hover:shadow-xl transition-all">
                ZADD
              </button>
              <button onClick={handleZScore} className="px-6 py-3 text-sm font-semibold text-gray-700 bg-gray-100 rounded-xl hover:bg-gray-200 transition-colors">
                ZSCORE
              </button>
              <button onClick={handleZRange} className="px-6 py-3 text-sm font-semibold text-gray-700 bg-gray-100 rounded-xl hover:bg-gray-200 transition-colors">
                ZRANGE
              </button>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
