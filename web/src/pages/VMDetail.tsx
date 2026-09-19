import React, { useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import client from '../api/client';
import { VM } from '../api/types';

const fetchVM = async (id: string): Promise<VM> => {
  const res = await client.get(`/vms/${id}`);
  return res.data;
};

const VMDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const { data: vm, isLoading, error } = useQuery({
    queryKey: ['vm', id],
    queryFn: () => fetchVM(id!),
    enabled: !!id,
  });

  const actionMutation = useMutation({
    mutationFn: async ({ action, hostId }: { action: string; hostId?: string }) => {
      const url = action === 'migrate'
        ? `/vms/${id}/migrate?host_id=${hostId}`
        : `/vms/${id}/${action}`;
      const res = await client.post(url);
      return res.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['vm', id] });
      queryClient.invalidateQueries({ queryKey: ['vms'] });
    },
  });

  const handleAction = async (action: string, hostId?: string) => {
    setActionLoading(action);
    try {
      await actionMutation.mutateAsync({ action, hostId });
    } catch (err) {
      console.error(`Failed to ${action} VM:`, err);
    } finally {
      setActionLoading(null);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading VM details...</div>
      </div>
    );
  }

  if (error || !vm) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load VM details
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-slate-400">
        <Link to="/vms" className="hover:text-blue-400">VMs</Link>
        <span>/</span>
        <span className="text-slate-200">{vm.name}</span>
      </div>

      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-slate-100">{vm.name}</h1>
        <div className="flex gap-2">
          <button
            onClick={() => handleAction('start')}
            disabled={actionLoading !== null || vm.status === 'running'}
            className="bg-green-600 hover:bg-green-700 disabled:bg-slate-600 text-white px-3 py-1.5 rounded text-sm font-medium transition-colors"
          >
            {actionLoading === 'start' ? 'Starting...' : 'Start'}
          </button>
          <button
            onClick={() => handleAction('stop')}
            disabled={actionLoading !== null || vm.status === 'stopped'}
            className="bg-red-600 hover:bg-red-700 disabled:bg-slate-600 text-white px-3 py-1.5 rounded text-sm font-medium transition-colors"
          >
            {actionLoading === 'stop' ? 'Stopping...' : 'Stop'}
          </button>
          <button
            onClick={() => handleAction('restart')}
            disabled={actionLoading !== null}
            className="bg-yellow-600 hover:bg-yellow-700 disabled:bg-slate-600 text-white px-3 py-1.5 rounded text-sm font-medium transition-colors"
          >
            {actionLoading === 'restart' ? 'Restarting...' : 'Restart'}
          </button>
          <button
            onClick={() => handleAction('migrate')}
            disabled={actionLoading !== null}
            className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-3 py-1.5 rounded text-sm font-medium transition-colors"
          >
            {actionLoading === 'migrate' ? 'Migrating...' : 'Migrate'}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Status</h3>
          <span className={`inline-block mt-1 px-2 py-0.5 rounded text-sm font-medium ${
            vm.status === 'running' ? 'bg-green-900/50 text-green-300' :
            vm.status === 'stopped' ? 'bg-red-900/50 text-red-300' :
            'bg-yellow-900/50 text-yellow-300'
          }`}>
            {vm.status}
          </span>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Host</h3>
          <Link to={`/hosts/${vm.host_id}`} className="text-blue-400 hover:text-blue-300 mt-1 block font-medium">
            {vm.host_id}
          </Link>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Role</h3>
          <p className="text-slate-100 mt-1 font-medium">{vm.role}</p>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">CPUs</h3>
          <p className="text-slate-100 mt-1 font-medium">{vm.cpus} cores</p>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Memory</h3>
          <p className="text-slate-100 mt-1 font-medium">{(vm.memory_bytes / (1024 ** 3)).toFixed(1)} GB</p>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg border border-slate-700">
          <h3 className="text-slate-400 text-sm">Memory Reservation</h3>
          <p className="text-slate-100 mt-1 font-medium">{(vm.memory_reservation_bytes / (1024 ** 3)).toFixed(1)} GB</p>
        </div>
      </div>

      <div className="bg-slate-800 p-6 rounded-lg border border-slate-700">
        <h2 className="text-lg font-semibold text-slate-100 mb-4">HANA Configuration</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="flex justify-between text-slate-300">
            <span>NUMA Policy</span>
            <span className="font-medium">{vm.numa_policy}</span>
          </div>
          <div className="flex justify-between text-slate-300">
            <span>Hugepages</span>
            <span className={`font-medium ${vm.hugepages_enabled ? 'text-green-400' : 'text-slate-400'}`}>
              {vm.hugepages_enabled ? 'Enabled' : 'Disabled'}
            </span>
          </div>
          <div className="flex justify-between text-slate-300">
            <span>Ballooning</span>
            <span className={`font-medium ${vm.ballooning_allowed ? 'text-yellow-400' : 'text-green-400'}`}>
              {vm.ballooning_allowed ? 'Allowed' : 'Disabled'}
            </span>
          </div>
          <div className="flex justify-between text-slate-300">
            <span>Swap</span>
            <span className={`font-medium ${vm.swap_allowed ? 'text-yellow-400' : 'text-green-400'}`}>
              {vm.swap_allowed ? 'Allowed' : 'Disabled'}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default VMDetail;
