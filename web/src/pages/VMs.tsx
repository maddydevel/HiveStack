import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import client from '../api/client';
import { VM } from '../api/types';

const fetchVMs = async (): Promise<VM[]> => {
  const res = await client.get('/vms');
  return res.data;
};

const formatBytes = (bytes: number): string => {
  const gb = bytes / (1024 ** 3);
  return gb >= 1024 ? `${(gb / 1024).toFixed(1)} TB` : `${gb.toFixed(1)} GB`;
};

const VMs: React.FC = () => {
  const { data: vms, isLoading, error } = useQuery({
    queryKey: ['vms'],
    queryFn: fetchVMs,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading VMs...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load VMs
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-slate-100">Virtual Machines</h1>
        <Link
          to="/vms/create"
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded font-medium transition-colors"
        >
          Create VM
        </Link>
      </div>

      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-700/50">
            <tr>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Name</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Host</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Status</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Role</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">CPUs</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Memory</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {vms?.map((vm: VM) => (
              <tr key={vm.id} className="hover:bg-slate-700/30">
                <td className="px-4 py-3">
                  <Link to={`/vms/${vm.id}`} className="text-blue-400 hover:text-blue-300 font-medium">
                    {vm.name}
                  </Link>
                </td>
                <td className="px-4 py-3 text-slate-300">{vm.host_id}</td>
                <td className="px-4 py-3">
                  <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                    vm.status === 'running' ? 'bg-green-900/50 text-green-300' :
                    vm.status === 'stopped' ? 'bg-red-900/50 text-red-300' :
                    'bg-yellow-900/50 text-yellow-300'
                  }`}>
                    {vm.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-slate-300">{vm.role}</td>
                <td className="px-4 py-3 text-slate-300">{vm.cpus}</td>
                <td className="px-4 py-3 text-slate-300">{formatBytes(vm.memory_bytes)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {vms?.length === 0 && (
          <div className="text-center py-8 text-slate-400">No VMs found</div>
        )}
      </div>
    </div>
  );
};

export default VMs;
