'use client';

import { useState, useEffect } from 'react';
import { RefreshCw, Plus, Search, Key, Clock, Copy, Trash2 } from 'lucide-react';
import { getKeys, executeCommand } from '@/lib/api';
import { useToast } from '@/context/ToastContext';
import Modal from './Modal';

interface KeyDetails {
  key: string;
  value: string | null;
  ttl: number;
  type: string;
}

export default function KeysBrowser() {
  const [keys, setKeys] = useState<string[]>([]);
  const [filteredKeys, setFilteredKeys] = useState<string[]>([]);
  const [search, setSearch] = useState('');
  const [selectedKey, setSelectedKey] = useState<KeyDetails | null>(null);
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKey, setNewKey] = useState({ name: '', value: '' });
  const { showToast } = useToast();

  async function loadKeys() {
    const data = await getKeys();
    setKeys(data);
  }

  useEffect(() => {
    loadKeys();
  }, []);

  useEffect(() => {
    setFilteredKeys(
      keys.filter((key) => key.toLowerCase().includes(search.toLowerCase()))
    );
  }, [keys, search]);

  const selectKey = async (key: string) => {
    const valueRes = await executeCommand(`GET ${key}`);
    const ttlRes = await executeCommand(`TTL ${key}`);

    const value =
      typeof valueRes.result === 'string'
        ? valueRes.result
        : valueRes.result == null
        ? null
        : JSON.stringify(valueRes.result);

    const ttlValue =
      typeof ttlRes.result === 'number'
        ? ttlRes.result
        : Number(ttlRes.result);

    setSelectedKey({
      key,
      value,
      ttl: Number.isFinite(ttlValue) ? ttlValue : -1,
      type: 'string',
    });
  };

  const copyValue = () => {
    if (selectedKey?.value) {
      navigator.clipboard.writeText(selectedKey.value);
      showToast('Value copied to clipboard', 'success');
    }
  };

  const deleteKey = async () => {
    if (!selectedKey) return;
    
    if (confirm(`Delete key "${selectedKey.key}"?`)) {
      await executeCommand(`DEL ${selectedKey.key}`);
      showToast('Key deleted', 'success');
      setSelectedKey(null);
      loadKeys();
    }
  };

  const addKey = async () => {
    if (!newKey.name || !newKey.value) {
      showToast('Please fill in both fields', 'error');
      return;
    }
    
    await executeCommand(`SET ${newKey.name} "${newKey.value}"`);
    showToast('Key created', 'success');
    setShowAddModal(false);
    setNewKey({ name: '', value: '' });
    loadKeys();
  };

  const formatTTL = (ttl: number) => {
    if (ttl < 0) return 'No expiry';
    if (ttl < 60) return `${ttl}s`;
    if (ttl < 3600) return `${Math.floor(ttl / 60)}m`;
    return `${Math.floor(ttl / 3600)}h`;
  };

  return (
    <section id="keys" className="mb-16">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-3xl font-bold tracking-tight">Keys Browser</h2>
        <div className="flex gap-3">
          <button
            onClick={loadKeys}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-600 bg-white border border-gray-200 rounded-full hover:bg-gray-50 transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
          <button
            onClick={() => setShowAddModal(true)}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-gradient-to-r from-blue-600 to-cyan-500 rounded-full shadow-lg shadow-blue-500/25 hover:shadow-xl transition-all"
          >
            <Plus className="w-4 h-4" />
            Add Key
          </button>
        </div>
      </div>

      {/* Browser Grid */}
      <div className="grid md:grid-cols-5 gap-6">
        {/* Keys List */}
        <div className="md:col-span-2 bg-white border border-gray-200 rounded-3xl p-5 shadow-md h-[500px] flex flex-col">
          {/* Search */}
          <div className="relative mb-4">
            <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search keys..."
              className="w-full pl-11 pr-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
            />
          </div>

          {/* Keys */}
          <div className="flex-1 overflow-y-auto space-y-2">
            {filteredKeys.length === 0 ? (
              <div className="text-center text-gray-400 py-10">
                {keys.length === 0 ? 'No keys yet. Create your first key!' : 'No matching keys'}
              </div>
            ) : (
              filteredKeys.map((key) => (
                <button
                  key={key}
                  onClick={() => selectKey(key)}
                  className={`w-full flex items-center justify-between px-4 py-3 rounded-xl text-left transition-all ${
                    selectedKey?.key === key
                      ? 'bg-gradient-to-r from-blue-600 to-cyan-500 text-white shadow-lg shadow-blue-500/25'
                      : 'hover:bg-gray-100 border border-transparent hover:border-gray-200'
                  }`}
                >
                  <span className="font-medium text-sm truncate">{key}</span>
                  <span
                    className={`text-xs px-2 py-1 rounded-full ${
                      selectedKey?.key === key
                        ? 'bg-white/20'
                        : 'bg-gray-100 text-gray-500'
                    }`}
                  >
                    string
                  </span>
                </button>
              ))
            )}
          </div>
        </div>

        {/* Key Details */}
        <div className="md:col-span-3 bg-white border border-gray-200 rounded-3xl p-8 shadow-md">
          {selectedKey ? (
            <>
              <h3 className="text-2xl font-bold mb-6">Key Details</h3>
              
              <div className="space-y-5">
                <div className="flex justify-between py-4 border-b border-gray-100">
                  <span className="text-gray-500 flex items-center gap-2">
                    <Key className="w-4 h-4" /> Key
                  </span>
                  <span className="font-mono font-semibold">{selectedKey.key}</span>
                </div>
                
                <div className="flex justify-between py-4 border-b border-gray-100">
                  <span className="text-gray-500">Type</span>
                  <span className="font-semibold">{selectedKey.type}</span>
                </div>
                
                <div className="flex justify-between py-4 border-b border-gray-100">
                  <span className="text-gray-500">Value</span>
                  <span className="font-mono font-semibold max-w-xs truncate">
                    {selectedKey.value ?? '(nil)'}
                  </span>
                </div>
                
                <div className="flex justify-between py-4 border-b border-gray-100">
                  <span className="text-gray-500 flex items-center gap-2">
                    <Clock className="w-4 h-4" /> TTL
                  </span>
                  <span className="font-semibold">{formatTTL(selectedKey.ttl)}</span>
                </div>
              </div>

              <div className="flex gap-3 mt-8">
                <button
                  onClick={copyValue}
                  className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-600 bg-gray-100 rounded-full hover:bg-gray-200 transition-colors"
                >
                  <Copy className="w-4 h-4" />
                  Copy Value
                </button>
                <button
                  onClick={deleteKey}
                  className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-600 bg-red-50 rounded-full hover:bg-red-100 transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                  Delete
                </button>
              </div>
            </>
          ) : (
            <div className="h-full flex flex-col items-center justify-center text-gray-400">
              <Key className="w-16 h-16 mb-4 opacity-30" />
              <p className="text-lg font-medium">No Key Selected</p>
              <p className="text-sm">Select a key from the list to view details</p>
            </div>
          )}
        </div>
      </div>

      {/* Add Key Modal */}
      <Modal isOpen={showAddModal} onClose={() => setShowAddModal(false)} title="Add New Key">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Key Name</label>
            <input
              type="text"
              value={newKey.name}
              onChange={(e) => setNewKey({ ...newKey, name: e.target.value })}
              placeholder="Enter key name"
              className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Value</label>
            <input
              type="text"
              value={newKey.value}
              onChange={(e) => setNewKey({ ...newKey, value: e.target.value })}
              placeholder="Enter value"
              className="w-full px-4 py-3 bg-gray-100 border border-transparent rounded-xl text-sm focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 transition-all"
            />
          </div>
          <div className="flex gap-3 pt-4">
            <button
              onClick={addKey}
              className="flex-1 py-3 text-sm font-semibold text-white bg-gradient-to-r from-blue-600 to-cyan-500 rounded-xl shadow-lg shadow-blue-500/25 hover:shadow-xl transition-all"
            >
              Create Key
            </button>
            <button
              onClick={() => setShowAddModal(false)}
              className="flex-1 py-3 text-sm font-semibold text-gray-700 bg-gray-100 rounded-xl hover:bg-gray-200 transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      </Modal>
    </section>
  );
}
