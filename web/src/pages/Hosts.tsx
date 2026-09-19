import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import client from '../api/client';
import { Host } from '../api/types';

const fetchHosts = async (): Promise<Host[]> => {
  const res = await client.get('/hosts');
  return res.data;
};

const formatBytes = (bytes: number): string => {
  const gb = bytes / (1024 ** 3);
  return gb >= 1024 ? `${(gb / 1024).toFixed(1)} TB` : `${gb.toFixed(1)} GB`;
};

const Hosts: React.FC = () => {
  const { data: hosts, isLoading, error } = useQuery({
    queryKey: ['hosts'],
    queryFn: fetchHosts,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading hosts...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load hosts
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-slate-100">Hosts</h1>
      </div>

      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-700/50">
            <tr>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Name</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Address</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Status</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">CPUs</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Memory</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">VM Count</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {hosts?.map((host: Host) => (
              <tr key={host.id} className="hover:bg-slate-700/30">
                <td className="px-4 py-3">
                  <Link to={`/hosts/${host.id}`} className="text-blue-400 hover:text-blue-300 font-medium">
                    {host.name}
                  </Link>
                </td>
                <td className="px-4 py-3 text-slate-300">{host.address}</td>
                <td className="px-4 py-3">
                  <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                    host.status === 'online' ? 'bg-green-900/50 text-green-300' :
                    host.status === 'offline' ? 'bg-red-900/50 text-red-300' :
                    'bg-yellow-900/50 text-yellow-300'
                  }`}>
                    {host.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-slate-300">{host.cpus}</td>
                <td className="px-4 py-3 text-slate-300">{formatBytes(host.memory_bytes)}</td>
                <td className="px-4 py-3 text-slate-300">{host.vm_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {hosts?.length === 0 && (
          <div className="text-center py-8 text-slate-400">No hosts found</div>
        )}
      </div>
    </div>
  );
};

export default Hosts;
