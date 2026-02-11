'use client';

import { useState } from 'react';
import { Book, Zap, Target, Code, ArrowRight } from 'lucide-react';
import Modal from './Modal';

interface DocCardProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  onClick: () => void;
}

function DocCard({ icon, title, description, onClick }: DocCardProps) {
  return (
    <button
      onClick={onClick}
      className="group bg-white border border-gray-200 rounded-2xl p-6 text-left shadow-sm hover:shadow-lg hover:-translate-y-1 hover:border-blue-500/30 transition-all"
    >
      <div className="w-12 h-12 bg-gradient-to-br from-blue-500 to-cyan-400 rounded-xl flex items-center justify-center shadow-lg shadow-blue-500/20 mb-4">
        {icon}
      </div>
      <h3 className="text-lg font-semibold mb-2">{title}</h3>
      <p className="text-sm text-gray-500 mb-4 leading-relaxed">{description}</p>
      <span className="inline-flex items-center gap-1 text-sm font-semibold text-blue-600 group-hover:gap-2 transition-all">
        Learn more <ArrowRight className="w-4 h-4" />
      </span>
    </button>
  );
}

export default function Documentation() {
  const [modalContent, setModalContent] = useState<{ title: string; content: React.ReactNode } | null>(null);

  const quickStartContent = (
    <div className="space-y-4 text-gray-600">
      <h4 className="font-semibold text-gray-900">Getting Started with FlashDB</h4>
      <ol className="space-y-3 list-decimal list-inside">
        <li>Use the console above to execute commands directly</li>
        <li>Create your first key: <code className="px-2 py-1 bg-gray-100 rounded text-sm">SET mykey &quot;hello&quot;</code></li>
        <li>Retrieve the value: <code className="px-2 py-1 bg-gray-100 rounded text-sm">GET mykey</code></li>
        <li>Explore the Keys Browser to manage your data visually</li>
        <li>Try the Feature Lab for guided operations</li>
      </ol>
    </div>
  );

  const commandRefContent = (
    <div className="space-y-4 text-gray-600">
      <h4 className="font-semibold text-gray-900">Available Commands</h4>
      <div className="space-y-3">
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-blue-600">SET key value</code>
          <p className="text-sm mt-1">Set a key to hold a string value</p>
        </div>
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-blue-600">GET key</code>
          <p className="text-sm mt-1">Get the value of a key</p>
        </div>
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-blue-600">DEL key</code>
          <p className="text-sm mt-1">Delete a key</p>
        </div>
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-blue-600">INCR/DECR key</code>
          <p className="text-sm mt-1">Increment or decrement the integer value</p>
        </div>
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-blue-600">EXPIRE key seconds</code>
          <p className="text-sm mt-1">Set a key&apos;s time to live in seconds</p>
        </div>
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-blue-600">ZADD key score member</code>
          <p className="text-sm mt-1">Add member to sorted set</p>
        </div>
      </div>
    </div>
  );

  const bestPracticesContent = (
    <div className="space-y-4 text-gray-600">
      <h4 className="font-semibold text-gray-900">Tips for Optimal Performance</h4>
      <ul className="space-y-3">
        <li className="flex gap-3">
          <span className="w-6 h-6 bg-green-100 text-green-600 rounded-full flex items-center justify-center text-sm flex-shrink-0">✓</span>
          <span>Use meaningful key names with consistent naming conventions</span>
        </li>
        <li className="flex gap-3">
          <span className="w-6 h-6 bg-green-100 text-green-600 rounded-full flex items-center justify-center text-sm flex-shrink-0">✓</span>
          <span>Set appropriate TTL values for temporary data</span>
        </li>
        <li className="flex gap-3">
          <span className="w-6 h-6 bg-green-100 text-green-600 rounded-full flex items-center justify-center text-sm flex-shrink-0">✓</span>
          <span>Use counters for tracking metrics and statistics</span>
        </li>
        <li className="flex gap-3">
          <span className="w-6 h-6 bg-green-100 text-green-600 rounded-full flex items-center justify-center text-sm flex-shrink-0">✓</span>
          <span>Leverage sorted sets for leaderboards and rankings</span>
        </li>
        <li className="flex gap-3">
          <span className="w-6 h-6 bg-green-100 text-green-600 rounded-full flex items-center justify-center text-sm flex-shrink-0">✓</span>
          <span>Monitor memory usage regularly</span>
        </li>
      </ul>
    </div>
  );

  const apiDocContent = (
    <div className="space-y-4 text-gray-600">
      <h4 className="font-semibold text-gray-900">HTTP API Endpoints</h4>
      <div className="space-y-3">
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-purple-600">POST /api/v1/execute</code>
          <p className="text-sm mt-1">Execute a FlashDB command</p>
        </div>
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-purple-600">GET /api/v1/stats</code>
          <p className="text-sm mt-1">Get server information</p>
        </div>
        <div className="p-3 bg-gray-50 rounded-lg">
          <code className="font-semibold text-purple-600">GET /api/v1/keys</code>
          <p className="text-sm mt-1">List all keys</p>
        </div>
      </div>
      <h4 className="font-semibold text-gray-900 mt-6">Example Request</h4>
      <pre className="p-4 bg-gray-900 text-green-400 rounded-lg text-sm overflow-x-auto">
{`curl -X POST http://localhost:8080/api/v1/execute \\
  -H "Content-Type: application/json" \\
  -d '{"command": "SET mykey value"}'`}
      </pre>
    </div>
  );

  return (
    <section id="docs">
      <h2 className="text-3xl font-bold tracking-tight mb-6">Documentation & Help</h2>

      <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-5">
        <DocCard
          icon={<Book className="w-6 h-6 text-white" />}
          title="Getting Started"
          description="Learn the basics of FlashDB and start building your first application."
          onClick={() => setModalContent({ title: 'Quick Start Guide', content: quickStartContent })}
        />
        <DocCard
          icon={<Zap className="w-6 h-6 text-white" />}
          title="Command Reference"
          description="Complete list of all available FlashDB commands with examples."
          onClick={() => setModalContent({ title: 'Command Reference', content: commandRefContent })}
        />
        <DocCard
          icon={<Target className="w-6 h-6 text-white" />}
          title="Best Practices"
          description="Tips and recommendations for optimal performance."
          onClick={() => setModalContent({ title: 'Best Practices', content: bestPracticesContent })}
        />
        <DocCard
          icon={<Code className="w-6 h-6 text-white" />}
          title="API Documentation"
          description="Integrate FlashDB into your applications using our HTTP API."
          onClick={() => setModalContent({ title: 'API Documentation', content: apiDocContent })}
        />
      </div>

      <Modal
        isOpen={modalContent !== null}
        onClose={() => setModalContent(null)}
        title={modalContent?.title || ''}
      >
        {modalContent?.content}
      </Modal>
    </section>
  );
}
