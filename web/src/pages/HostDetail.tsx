import React from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import client from '../api/client';
import { Host, VM } from '../api/types';

const fetchHost = async (id: string): Promise<Host> => {
  const res = await client.get(`/hosts/${id}`);
  return res.data;
};

const fetchHostVMs = async (id: string): Promise<VM[]> => {
  const res = await client.get('/vms');
  return res.data.filter((vm: VM) => vm.host_id === id);
};

const formatBytes = (bytes: number): string => {
  const gb = bytes / (1024 ** 3);
  return gb >= 1024 ? `${(gb / 1024).toFixed(1)} TB` : `${gb.toFixed(1)} GB`;
};

const HostDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();

  const { data: host, isLoading: hostLoading, error: hostError } = useQuery({
    queryKey: ['host', id],
    queryFn: () => fetchHost(id!),
    enabled: !!id,
  });

  const { data: vms, isLoading: vmsLoading } = useQuery({
    queryKey: ['host-vms', id],
    queryFn: () => fetchHostVMs(id!),
    enabled: !!id,
  });

  if (hostLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading host details...</div>
      </div>
    );
  }

  if (hostError || !host) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load host details
      </div>
    );
  }

  const totalVMMemory = vms?.reduce((sum: number, vm: VM) => sum + vm.memory_bytes, 0) ?? 0;
  const usedMemoryPercentage = (totalVMMemory / host.memory_bytes) * 100;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-slate-400">
        <Link to="/hosts" className="hover:text-blue-400">Hosts</Link>
        <span>/</span>
        <span className="text-slate-200">{host.name}</span>
      </div>

      <h1 className="text-2xl font-bold text-slate-100">{host.name}</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Status</h3>
          <span className={`inline-block mt-1 px-2 py-0.5 rounded text-sm font-medium ${
            host.status === 'online' ? 'bg-green-900/50 text-green-300' :
            host.status === 'offline' ? 'bg-red-900/50 text-red-300' :
            'bg-yellow-900/50 text-yellow-300'
          }`}>
            {host.status}
          </span>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Address</h3>
          <p className="text-slate-100 mt-1 font-medium">{host.address}</p>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">CPUs</h3>
          <p className="text-slate-100 mt-1 font-medium">{host.cpus} cores</p>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Total Memory</h3>
          <p className="text-slate-100 mt-1 font-medium">{formatBytes(host.memory_bytes)}</p>
        </div>
      </div>

      <div className="bg-slate-800 p-6 rounded-lg border border-slate-700">
        <h2 className="text-lg font-semibold text-slate-100 mb-4">Resource Usage</h2>
        <div className="space-y-4">
          <div>
            <div className="flex justify-between text-sm text-slate-300 mb-1">
              <span>Memory Allocated to VMs</span>
              <span>{formatBytes(totalVMMemory)} / {formatBytes(host.memory_bytes)}</span>
            </div>
            <div className="w-full bg-slate-700 rounded-full h-2">
              <div
                className="bg-blue-500 h-2 rounded-full"
                style={{ width: `${Math.min(usedMemoryPercentage, 100)}%` }}
              />
            </div>
            <p className="text-xs text-slate-400 mt-1">{usedMemoryPercentage.toFixed(1)}% utilized</p>
          </div>
        </div>
      </div>

      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-700">
          <h2 className="text-lg font-semibold text-slate-100">Virtual Machines ({host.vm_count})</h2>
        </div>
        {vmsLoading ? (
          <div className="text-center py-8 text-slate-400">Loading VMs...</div>
        ) : vms && vms.length > 0 ? (
          <table className="w-full">
            <thead className="bg-slate-700/50">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Name</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Status</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Role</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">CPUs</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Memory</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-700">
              {vms.map((vm: VM) => (
                <tr key={vm.id} className="hover:bg-slate-700/30">
                  <td className="px-4 py-3">
                    <Link to={`/vms/${vm.id}`} className="text-blue-400 hover:text-blue-300 font-medium">
                      {vm.name}
                    </Link>
                  </td>
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
        ) : (
          <div className="text-center py-8 text-slate-400">No VMs on this host</div>
        )}
      </div>
    </div>
  );
};

export default HostDetail;
