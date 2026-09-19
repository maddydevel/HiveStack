import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import client from '../api/client';
import { Host, CreateVMRequest } from '../api/types';

const fetchHosts = async (): Promise<Host[]> => {
  const res = await client.get('/hosts');
  return res.data;
};

const VMCreate: React.FC = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: hosts, isLoading: hostsLoading } = useQuery({
    queryKey: ['hosts'],
    queryFn: fetchHosts,
  });

  const [formData, setFormData] = useState<CreateVMRequest>({
    name: '',
    host_id: '',
    cpus: 4,
    memory_bytes: 16 * 1024 ** 3,
    memory_reservation_bytes: 16 * 1024 ** 3,
    ballooning_allowed: false,
    swap_allowed: false,
    hugepages_enabled: true,
    numa_policy: 'interleave',
  });

  const [error, setError] = useState('');

  const createMutation = useMutation({
    mutationFn: async (data: CreateVMRequest) => {
      const res = await client.post('/vms', data);
      return res.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['vms'] });
      navigate('/vms');
    },
    onError: (err: any) => {
      setError(err.response?.data?.message || 'Failed to create VM');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    createMutation.mutate(formData);
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value, type } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: type === 'checkbox' ? (e.target as HTMLInputElement).checked : value,
    }));
  };

  const formatMemoryGB = (bytes: number): number => bytes / (1024 ** 3);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Create Virtual Machine</h1>

      {error && (
        <div className="bg-red-900/30 border border-red-500 text-red-200 px-4 py-3 rounded">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="bg-slate-800 p-6 rounded-lg border border-slate-700">
          <h2 className="text-lg font-semibold text-slate-100 mb-4">Basic Configuration</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-slate-300 text-sm font-medium mb-2">VM Name</label>
              <input
                type="text"
                name="name"
                value={formData.name}
                onChange={handleChange}
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
                required
              />
            </div>
            <div>
              <label className="block text-slate-300 text-sm font-medium mb-2">Target Host</label>
              <select
                name="host_id"
                value={formData.host_id}
                onChange={handleChange}
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
                required
              >
                <option value="">Select a host</option>
                {hosts?.map((host) => (
                  <option key={host.id} value={host.id}>
                    {host.name} ({host.address})
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>

        <div className="bg-slate-800 p-6 rounded-lg border border-slate-700">
          <h2 className="text-lg font-semibold text-slate-100 mb-4">Compute Resources</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-slate-300 text-sm font-medium mb-2">CPUs</label>
              <input
                type="number"
                name="cpus"
                value={formData.cpus}
                onChange={handleChange}
                min="1"
                max="256"
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
                required
              />
            </div>
            <div>
              <label className="block text-slate-300 text-sm font-medium mb-2">Memory (GB)</label>
              <input
                type="number"
                name="memory_bytes"
                value={formatMemoryGB(formData.memory_bytes)}
                onChange={(e) =>
                  setFormData((prev) => ({
                    ...prev,
                    memory_bytes: parseInt(e.target.value) * 1024 ** 3,
                  }))
                }
                min="1"
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
                required
              />
            </div>
          </div>
        </div>

        <div className="bg-slate-800 p-6 rounded-lg border border-slate-700">
          <h2 className="text-lg font-semibold text-slate-100 mb-4">HANA Guardrails</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-slate-300 text-sm font-medium mb-2">Memory Reservation (GB)</label>
              <input
                type="number"
                name="memory_reservation_bytes"
                value={formatMemoryGB(formData.memory_reservation_bytes)}
                onChange={(e) =>
                  setFormData((prev) => ({
                    ...prev,
                    memory_reservation_bytes: parseInt(e.target.value) * 1024 ** 3,
                  }))
                }
                min="1"
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
                required
              />
            </div>
            <div>
              <label className="block text-slate-300 text-sm font-medium mb-2">NUMA Policy</label>
              <select
                name="numa_policy"
                value={formData.numa_policy}
                onChange={handleChange}
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-slate-100 focus:outline-none focus:border-blue-500"
              >
                <option value="interleave">Interleave</option>
                <option value="preferred">Preferred</option>
                <option value="bind">Bind</option>
              </select>
            </div>
          </div>

          <div className="mt-4 space-y-3">
            <label className="flex items-center gap-3 text-slate-300">
              <input
                type="checkbox"
                name="hugepages_enabled"
                checked={formData.hugepages_enabled}
                onChange={handleChange}
                className="w-4 h-4 rounded bg-slate-700 border-slate-600"
              />
              <span>Enable Hugepages (required for HANA)</span>
            </label>
            <label className="flex items-center gap-3 text-slate-300">
              <input
                type="checkbox"
                name="ballooning_allowed"
                checked={formData.ballooning_allowed}
                onChange={handleChange}
                className="w-4 h-4 rounded bg-slate-700 border-slate-600"
              />
              <span>Allow Memory Ballooning (not recommended for HANA)</span>
            </label>
            <label className="flex items-center gap-3 text-slate-300">
              <input
                type="checkbox"
                name="swap_allowed"
                checked={formData.swap_allowed}
                onChange={handleChange}
                className="w-4 h-4 rounded bg-slate-700 border-slate-600"
              />
              <span>Allow Swap (not recommended for HANA)</span>
            </label>
          </div>
        </div>

        <div className="flex gap-4">
          <button
            type="submit"
            disabled={createMutation.isPending}
            className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-6 py-2 rounded font-medium transition-colors"
          >
            {createMutation.isPending ? 'Creating...' : 'Create VM'}
          </button>
          <button
            type="button"
            onClick={() => navigate('/vms')}
            className="bg-slate-700 hover:bg-slate-600 text-slate-200 px-6 py-2 rounded font-medium transition-colors"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
};

export default VMCreate;
