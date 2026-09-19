import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import client from '../api/client';
import { Backup, VM } from '../api/types';

const fetchBackups = async (): Promise<Backup[]> => {
  const res = await client.get('/backups');
  return res.data;
};

const fetchVMs = async (): Promise<VM[]> => {
  const res = await client.get('/vms');
  return res.data;
};

const Backups: React.FC = () => {
  const queryClient = useQueryClient();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [selectedVMId, setSelectedVMId] = useState('');
  const [backupName, setBackupName] = useState('');

  const { data: backups, isLoading, error } = useQuery({
    queryKey: ['backups'],
    queryFn: fetchBackups,
  });

  const { data: vms } = useQuery({
    queryKey: ['vms'],
    queryFn: fetchVMs,
  });

  const createMutation = useMutation({
    mutationFn: async () => {
      const res = await client.post('/backups', {
        vm_id: selectedVMId,
        name: backupName || `backup-${Date.now()}`,
      });
      return res.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backups'] });
      setShowCreateModal(false);
      setSelectedVMId('');
      setBackupName('');
    },
  });

  const restoreMutation = useMutation({
    mutationFn: async (backupId: string) => {
      const res = await client.post(`/backups/${backupId}/restore`);
      return res.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backups'] });
    },
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400 text-lg">Loading backups...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
        Failed to load backups
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-slate-100">Backups</h1>
        <button
          onClick={() => setShowCreateModal(true)}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded font-medium transition-colors"
        >
          Create Backup
        </button>
      </div>

      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-slate-800 p-6 rounded-lg border border-slate-700 w-full max-w-md">
            <h2 className="text-lg font-semibold text-slate-100 mb-4">Create Backup</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-slate-300 text-sm font-medium mb-2">Select VM</label>
                <select
                  value={selectedVMId}
                  onChange={(e) => setSelectedVMId(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
                >
                  <option value="">Choose a VM</option>
                  {vms?.map((vm) => (
                    <option key={vm.id} value={vm.id}>{vm.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-slate-300 text-sm font-medium mb-2">Backup Name (optional)</label>
                <input
                  type="text"
                  value={backupName}
                  onChange={(e) => setBackupName(e.target.value)}
                  placeholder="Auto-generated if empty"
                  className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
                />
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button
                onClick={() => createMutation.mutate()}
                disabled={!selectedVMId || createMutation.isPending}
                className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-4 py-2 rounded font-medium transition-colors"
              >
                {createMutation.isPending ? 'Creating...' : 'Create'}
              </button>
              <button
                onClick={() => setShowCreateModal(false)}
                className="bg-slate-700 hover:bg-slate-600 text-slate-200 px-4 py-2 rounded font-medium transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="bg-slate-800 rounded-lg border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-700/50">
            <tr>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Name</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">VM ID</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Status</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Created</th>
              <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {backups?.map((backup) => (
              <tr key={backup.id} className="hover:bg-slate-700/30">
                <td className="px-4 py-3 text-slate-100 font-medium">{backup.name}</td>
                <td className="px-4 py-3 text-slate-300 font-mono text-sm">{backup.vm_id}</td>
                <td className="px-4 py-3">
                  <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                    backup.status === 'completed' ? 'bg-green-900/50 text-green-300' :
                    backup.status === 'failed' ? 'bg-red-900/50 text-red-300' :
                    'bg-yellow-900/50 text-yellow-300'
                  }`}>
                    {backup.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-slate-300 text-sm">
                  {new Date(backup.created_at).toLocaleString()}
                </td>
                <td className="px-4 py-3">
                  <button
                    onClick={() => restoreMutation.mutate(backup.id)}
                    disabled={backup.status !== 'completed' || restoreMutation.isPending}
                    className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-3 py-1 rounded text-sm font-medium transition-colors"
                  >
                    Restore
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {backups?.length === 0 && (
          <div className="text-center py-8 text-slate-400">No backups found</div>
        )}
      </div>
    </div>
  );
};

export default Backups;
